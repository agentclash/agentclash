"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { OrgWorkspace } from "@/lib/api/types";
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
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Plus, Loader2, ChevronLeft, ChevronRight } from "lucide-react";
import { toast } from "sonner";
import Link from "next/link";

const PAGE_SIZE = 50;

interface OrgWorkspacesClientProps {
  orgId: string;
  isAdmin: boolean;
  initialWorkspaces: OrgWorkspace[];
  initialTotal: number;
}

export function OrgWorkspacesClient({
  orgId,
  isAdmin,
  initialWorkspaces,
  initialTotal,
}: OrgWorkspacesClientProps) {
  const { getAccessToken } = useAccessToken();
  const [workspaces, setWorkspaces] =
    useState<OrgWorkspace[]>(initialWorkspaces);
  const [total, setTotal] = useState(initialTotal);
  const [offset, setOffset] = useState(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();

  async function fetchWorkspaces(currentOffset: number) {
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      const res = await api.get<{ items: OrgWorkspace[]; total: number }>(
        `/v1/organizations/${orgId}/workspaces`,
        { params: { limit: PAGE_SIZE, offset: currentOffset } },
      );
      setWorkspaces(res.items);
      setTotal(res.total);
    } catch {
      // Silently fail
    }
  }

  function refreshWorkspaces() {
    fetchWorkspaces(offset);
  }

  async function handleCreate() {
    if (!newName.trim()) return;
    setCreateError(undefined);
    setCreating(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const api = createApiClient(token);
      await api.post(`/v1/organizations/${orgId}/workspaces`, {
        name: newName.trim(),
      });
      toast.success(`Created workspace "${newName.trim()}"`);
      setCreateOpen(false);
      setNewName("");
      refreshWorkspaces();
    } catch (err) {
      setCreateError(
        err instanceof ApiError ? err.message : "Failed to create workspace",
      );
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {total} workspace{total !== 1 ? "s" : ""}
        </p>
        {isAdmin && (
          <Dialog
            open={createOpen}
            onOpenChange={(next) => {
              setCreateOpen(next);
              if (next) {
                setNewName("");
                setCreateError(undefined);
              }
            }}
          >
            <DialogTrigger render={<Button size="sm" />}>
              <Plus className="size-4 mr-1.5" />
              Create Workspace
            </DialogTrigger>
            <DialogContent className="sm:max-w-sm">
              <DialogHeader>
                <DialogTitle>Create Workspace</DialogTitle>
                <DialogDescription>
                  Add a new workspace to this organization.
                </DialogDescription>
              </DialogHeader>
              <div>
                <label className="block text-sm font-medium mb-1.5">
                  Workspace Name
                </label>
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  disabled={creating}
                  placeholder="My Workspace"
                  className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50"
                />
                {createError && (
                  <p className="mt-1.5 text-xs text-destructive">
                    {createError}
                  </p>
                )}
              </div>
              <DialogFooter>
                <Button
                  onClick={handleCreate}
                  disabled={!newName.trim() || creating}
                >
                  {creating && (
                    <Loader2
                      data-icon="inline-start"
                      className="size-4 animate-spin"
                    />
                  )}
                  {creating ? "Creating..." : "Create"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </div>

      {workspaces.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          No workspaces yet.{isAdmin ? " Create one to get started." : ""}
        </div>
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workspaces.map((ws) => (
                <TableRow key={ws.id}>
                  <TableCell>
                    <Link
                      href={`/workspaces/${ws.id}`}
                      className="font-medium text-sm text-foreground hover:underline underline-offset-4"
                    >
                      {ws.name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs font-[family-name:var(--font-mono)] text-muted-foreground">
                      {ws.slug}
                    </code>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        ws.status === "active" ? "default" : "destructive"
                      }
                    >
                      {ws.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(ws.created_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Pagination */}
      {Math.ceil(total / PAGE_SIZE) > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Page {Math.floor(offset / PAGE_SIZE) + 1} of{" "}
            {Math.ceil(total / PAGE_SIZE)}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset === 0}
              onClick={() => {
                const newOffset = Math.max(0, offset - PAGE_SIZE);
                setOffset(newOffset);
                fetchWorkspaces(newOffset);
              }}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              variant="outline"
              size="icon-sm"
              disabled={offset + PAGE_SIZE >= total}
              onClick={() => {
                const newOffset = offset + PAGE_SIZE;
                if (newOffset < total) {
                  setOffset(newOffset);
                  fetchWorkspaces(newOffset);
                }
              }}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
