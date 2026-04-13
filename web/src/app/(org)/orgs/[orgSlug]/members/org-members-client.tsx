"use client";

import { useState, useCallback } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { OrgMember, OrgRole, OrgMembershipStatus } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { PaginationControls } from "@/components/ui/pagination-controls";
import { MoreHorizontal } from "lucide-react";
import { toast } from "sonner";
import { InviteMemberDialog } from "./invite-member-dialog";

const PAGE_SIZE = 50;

const roleBadgeVariant: Record<string, "default" | "secondary" | "outline"> = {
  org_admin: "default",
  org_member: "secondary",
};

const statusBadgeVariant: Record<
  string,
  "default" | "secondary" | "outline" | "destructive"
> = {
  active: "default",
  invited: "outline",
  suspended: "secondary",
  archived: "destructive",
};

function roleLabel(role: string): string {
  return role === "org_admin" ? "Admin" : "Member";
}

function statusLabel(status: string): string {
  return status.charAt(0).toUpperCase() + status.slice(1);
}

function isInviteExpired(member: OrgMember): boolean {
  if (member.membership_status !== "invited") return false;
  const timestamp = member.updated_at ?? member.created_at;
  const ts = new Date(timestamp).getTime();
  const sevenDays = 7 * 24 * 60 * 60 * 1000;
  return Date.now() - ts > sevenDays;
}

// --- Extracted action logic ---

interface MemberAction {
  label: string;
  onClick: () => void;
  variant?: "destructive";
  separator?: boolean;
}

function getMemberActions(
  member: OrgMember,
  isLastAdmin: boolean,
  onChangeRole: (role: OrgRole) => void,
  onChangeStatus: (status: OrgMembershipStatus) => void,
): MemberAction[] {
  const actions: MemberAction[] = [];
  const isArchived = member.membership_status === "archived";

  // Role changes
  if (!isArchived) {
    if (member.role === "org_member") {
      actions.push({ label: "Make Admin", onClick: () => onChangeRole("org_admin") });
    }
    if (member.role === "org_admin" && !isLastAdmin) {
      actions.push({ label: "Make Member", onClick: () => onChangeRole("org_member") });
    }
  }

  // Status changes
  if (member.membership_status === "active") {
    actions.push({ label: "Suspend", onClick: () => onChangeStatus("suspended"), separator: true });
  }
  if (member.membership_status === "suspended") {
    actions.push({ label: "Reactivate", onClick: () => onChangeStatus("active"), separator: true });
  }
  if (!isArchived) {
    actions.push({
      label: "Archive",
      onClick: () => onChangeStatus("archived"),
      variant: "destructive",
    });
  }

  return actions;
}

// --- Component ---

interface OrgMembersClientProps {
  orgId: string;
  isAdmin: boolean;
  currentUserId: string;
  initialMembers: OrgMember[];
  initialTotal: number;
}

export function OrgMembersClient({
  orgId,
  isAdmin,
  currentUserId,
  initialMembers,
  initialTotal,
}: OrgMembersClientProps) {
  const { getAccessToken } = useAccessToken();
  const [members, setMembers] = useState<OrgMember[]>(initialMembers);
  const [total, setTotal] = useState(initialTotal);
  const [offset, setOffset] = useState(0);

  const activeAdminCount = members.filter(
    (m) => m.role === "org_admin" && m.membership_status === "active",
  ).length;

  const fetchMembers = useCallback(
    async (currentOffset: number) => {
      try {
        const token = await getAccessToken();
        if (!token) return;
        const api = createApiClient(token);
        const res = await api.get<{
          items: OrgMember[];
          total: number;
        }>(`/v1/organizations/${orgId}/memberships`, {
          params: { limit: PAGE_SIZE, offset: currentOffset },
        });
        setMembers(res.items);
        setTotal(res.total);
      } catch {
        // Silently fail
      }
    },
    [getAccessToken, orgId],
  );

  function refreshMembers() {
    fetchMembers(offset);
  }

  async function handleChangeRole(member: OrgMember, newRole: OrgRole) {
    setMembers((prev) =>
      prev.map((m) => (m.id === member.id ? { ...m, role: newRole } : m)),
    );
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      await api.patch(`/v1/organization-memberships/${member.id}`, {
        role: newRole,
      });
      toast.success(
        `Changed ${member.display_name || member.email} to ${roleLabel(newRole)}`,
      );
      refreshMembers();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to change role",
      );
      refreshMembers();
    }
  }

  async function handleChangeStatus(
    member: OrgMember,
    newStatus: OrgMembershipStatus,
  ) {
    setMembers((prev) =>
      prev.map((m) =>
        m.id === member.id ? { ...m, membership_status: newStatus } : m,
      ),
    );
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      await api.patch(`/v1/organization-memberships/${member.id}`, {
        status: newStatus,
      });
      toast.success(
        `${statusLabel(newStatus)} ${member.display_name || member.email}`,
      );
      refreshMembers();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to update member",
      );
      refreshMembers();
    }
  }

  function canManageMember(member: OrgMember): boolean {
    if (!isAdmin) return false;
    if (member.user_id === currentUserId) return false;
    if (
      member.role === "org_admin" &&
      activeAdminCount <= 1 &&
      member.membership_status !== "suspended" &&
      member.membership_status !== "archived"
    )
      return false;
    return true;
  }

  function handlePageChange(newOffset: number) {
    setOffset(newOffset);
    fetchMembers(newOffset);
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {total} member{total !== 1 ? "s" : ""}
        </p>
        {isAdmin && (
          <InviteMemberDialog orgId={orgId} onInvited={refreshMembers} />
        )}
      </div>

      {members.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          No members found.
        </div>
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Member</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Joined</TableHead>
                {isAdmin && <TableHead className="w-12" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((member) => {
                const expired = isInviteExpired(member);
                const manageable = canManageMember(member);
                const isLastAdmin =
                  member.role === "org_admin" && activeAdminCount <= 1;
                const isInactive =
                  member.membership_status === "suspended" ||
                  member.membership_status === "archived";
                const actions = manageable
                  ? getMemberActions(
                      member,
                      isLastAdmin,
                      (role) => handleChangeRole(member, role),
                      (status) => handleChangeStatus(member, status),
                    )
                  : [];

                return (
                  <TableRow
                    key={member.id}
                    className={isInactive ? "opacity-60" : undefined}
                  >
                    <TableCell>
                      <div>
                        <span className="font-medium text-sm">
                          {member.display_name || member.email}
                        </span>
                        {member.display_name && (
                          <p className="text-xs text-muted-foreground">
                            {member.email}
                          </p>
                        )}
                        {member.user_id === currentUserId && (
                          <span className="text-xs text-muted-foreground ml-1">
                            (you)
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={roleBadgeVariant[member.role] ?? "outline"}
                      >
                        {roleLabel(member.role)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <Badge
                          variant={
                            statusBadgeVariant[member.membership_status] ??
                            "outline"
                          }
                        >
                          {statusLabel(member.membership_status)}
                        </Badge>
                        {expired && (
                          <span className="text-xs text-destructive">
                            Expired
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {new Date(member.created_at).toLocaleDateString()}
                    </TableCell>
                    {isAdmin && (
                      <TableCell>
                        {actions.length > 0 && (
                          <DropdownMenu>
                            <DropdownMenuTrigger
                              render={
                                <Button variant="ghost" size="icon-xs" />
                              }
                            >
                              <MoreHorizontal className="size-4" />
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {actions.map((action, i) => (
                                <span key={action.label}>
                                  {action.separator && i > 0 && (
                                    <DropdownMenuSeparator />
                                  )}
                                  <DropdownMenuItem
                                    className={
                                      action.variant === "destructive"
                                        ? "text-destructive"
                                        : undefined
                                    }
                                    onClick={action.onClick}
                                  >
                                    {action.label}
                                  </DropdownMenuItem>
                                </span>
                              ))}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      )}

      <PaginationControls
        offset={offset}
        total={total}
        pageSize={PAGE_SIZE}
        onPrev={() => handlePageChange(Math.max(0, offset - PAGE_SIZE))}
        onNext={() => {
          const next = offset + PAGE_SIZE;
          if (next < total) handlePageChange(next);
        }}
      />
    </div>
  );
}
