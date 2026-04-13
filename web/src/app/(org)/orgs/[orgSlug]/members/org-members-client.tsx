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
import { MoreHorizontal, ChevronLeft, ChevronRight } from "lucide-react";
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

// P2 fix: use updated_at (re-invite refresh) with fallback to created_at
function isInviteExpired(member: OrgMember): boolean {
  if (member.membership_status !== "invited") return false;
  const timestamp = member.updated_at ?? member.created_at;
  const ts = new Date(timestamp).getTime();
  const sevenDays = 7 * 24 * 60 * 60 * 1000;
  return Date.now() - ts > sevenDays;
}

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

  // Only count active admins for last-admin protection — invited admins
  // haven't accepted yet, so they shouldn't prevent demoting the last active one
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
    // P1 fix: optimistic update so the member stays visible
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
      refreshMembers(); // revert optimistic update
    }
  }

  async function handleChangeStatus(
    member: OrgMember,
    newStatus: OrgMembershipStatus,
  ) {
    // P1 fix: optimistic update keeps suspended/archived members visible
    // in the current page so the Reactivate action stays accessible
    setMembers((prev) =>
      prev.map((m) =>
        m.id === member.id
          ? { ...m, membership_status: newStatus }
          : m,
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
      // Re-fetch to get server state, but optimistic update keeps row visible
      // even if backend filters it out
      refreshMembers();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to update member",
      );
      refreshMembers(); // revert optimistic update
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

  // Pagination
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  function handlePrev() {
    const newOffset = Math.max(0, offset - PAGE_SIZE);
    setOffset(newOffset);
    fetchMembers(newOffset);
  }

  function handleNext() {
    const newOffset = offset + PAGE_SIZE;
    if (newOffset < total) {
      setOffset(newOffset);
      fetchMembers(newOffset);
    }
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {total} member{total !== 1 ? "s" : ""}
        </p>
        {isAdmin && (
          <InviteMemberDialog orgId={orgId} onInvited={refreshMembers} />
        )}
      </div>

      {/* Table */}
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
                        {manageable && (
                          <DropdownMenu>
                            <DropdownMenuTrigger
                              render={
                                <Button variant="ghost" size="icon-xs" />
                              }
                            >
                              <MoreHorizontal className="size-4" />
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {/* Role change */}
                              {member.role === "org_member" &&
                                member.membership_status !== "archived" && (
                                  <DropdownMenuItem
                                    onClick={() =>
                                      handleChangeRole(member, "org_admin")
                                    }
                                  >
                                    Make Admin
                                  </DropdownMenuItem>
                                )}
                              {member.role === "org_admin" &&
                                !isLastAdmin &&
                                member.membership_status !== "archived" && (
                                  <DropdownMenuItem
                                    onClick={() =>
                                      handleChangeRole(member, "org_member")
                                    }
                                  >
                                    Make Member
                                  </DropdownMenuItem>
                                )}
                              {member.membership_status !== "archived" && (
                                <DropdownMenuSeparator />
                              )}
                              {/* Status change */}
                              {member.membership_status === "active" && (
                                <DropdownMenuItem
                                  onClick={() =>
                                    handleChangeStatus(member, "suspended")
                                  }
                                >
                                  Suspend
                                </DropdownMenuItem>
                              )}
                              {member.membership_status === "suspended" && (
                                <DropdownMenuItem
                                  onClick={() =>
                                    handleChangeStatus(member, "active")
                                  }
                                >
                                  Reactivate
                                </DropdownMenuItem>
                              )}
                              {member.membership_status !== "archived" && (
                                <DropdownMenuItem
                                  className="text-destructive"
                                  onClick={() =>
                                    handleChangeStatus(member, "archived")
                                  }
                                >
                                  Archive
                                </DropdownMenuItem>
                              )}
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

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset === 0}
              onClick={handlePrev}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset + PAGE_SIZE >= total}
              onClick={handleNext}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
