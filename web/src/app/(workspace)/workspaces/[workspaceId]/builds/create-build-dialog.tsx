"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentBuild } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface CreateBuildDialogProps {
  workspaceId: string;
}

export function CreateBuildDialog({ workspaceId }: CreateBuildDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleCreate() {
    if (!name.trim()) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const build = await api.post<AgentBuild>(
        `/v1/workspaces/${workspaceId}/agent-builds`,
        {
          name: name.trim(),
          description: description.trim() || undefined,
        },
      );
      toast.success(`Created "${build.name}"`);
      setOpen(false);
      setName("");
      setDescription("");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to create agent build");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={<Button size="sm" />}
      >
        <Plus data-icon="inline-start" className="size-4" />
        New Agent Build
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Agent Build</DialogTitle>
          <DialogDescription>
            Define a new agent — you&apos;ll configure its behavior in a version after creating it.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Code Review Agent"
              autoFocus
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
              onKeyDown={(e) => {
                if (e.key === "Enter" && name.trim()) handleCreate();
              }}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Description{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What does this agent do?"
              rows={3}
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 resize-none"
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            disabled={!name.trim() || submitting}
            onClick={handleCreate}
          >
            {submitting ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Create"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
