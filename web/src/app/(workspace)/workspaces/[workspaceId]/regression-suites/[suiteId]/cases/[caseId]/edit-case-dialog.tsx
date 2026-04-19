"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Pencil } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  PatchRegressionCaseInput,
  RegressionCase,
  RegressionCaseStatus,
  RegressionSeverity,
} from "@/lib/api/types";
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
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const STATUS_OPTIONS: { value: RegressionCaseStatus; label: string }[] = [
  { value: "active", label: "Active" },
  { value: "muted", label: "Muted" },
  { value: "archived", label: "Archived" },
];

const SEVERITY_OPTIONS: RegressionSeverity[] = ["info", "warning", "blocking"];

interface EditCaseDialogProps {
  workspaceId: string;
  regressionCase: RegressionCase;
}

export function EditCaseDialog({
  workspaceId,
  regressionCase,
}: EditCaseDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [title, setTitle] = useState(regressionCase.title);
  const [description, setDescription] = useState(regressionCase.description);
  const [status, setStatus] = useState<RegressionCaseStatus>(
    regressionCase.status,
  );
  const [severity, setSeverity] = useState<RegressionSeverity>(
    regressionCase.severity,
  );
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setTitle(regressionCase.title);
      setDescription(regressionCase.description);
      setStatus(regressionCase.status);
      setSeverity(regressionCase.severity);
    }
  }, [open, regressionCase]);

  const archived = regressionCase.status === "archived";

  async function handleSave(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;

    const patch: PatchRegressionCaseInput = {};
    const trimmedTitle = title.trim();
    if (trimmedTitle !== regressionCase.title) patch.title = trimmedTitle;
    if (description !== regressionCase.description) {
      patch.description = description;
    }
    if (status !== regressionCase.status) patch.status = status;
    if (severity !== regressionCase.severity) patch.severity = severity;

    if (Object.keys(patch).length === 0) {
      setOpen(false);
      return;
    }
    if (patch.title !== undefined && !patch.title) {
      toast.error("Title cannot be empty");
      return;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.patch<RegressionCase>(
        `/v1/workspaces/${workspaceId}/regression-cases/${regressionCase.id}`,
        patch,
      );
      toast.success("Case updated");
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          toast.error("You don't have permission to edit this case.");
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to update case");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <Pencil data-icon="inline-start" className="size-4" />
        Edit
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit Case</DialogTitle>
          <DialogDescription>
            Title, description, status, and severity are editable here.
          </DialogDescription>
        </DialogHeader>

        <form
          id="edit-case-form"
          onSubmit={handleSave}
          className="space-y-4"
        >
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Title
            </label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </div>

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                Status
              </label>
              <Select
                value={status}
                onValueChange={(v) => v && setStatus(v as RegressionCaseStatus)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {STATUS_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                Severity
              </label>
              <Select
                value={severity}
                onValueChange={(v) => v && setSeverity(v as RegressionSeverity)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SEVERITY_OPTIONS.map((s) => (
                    <SelectItem key={s} value={s}>
                      {s}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {archived && (
            <p className="text-xs text-muted-foreground">
              Archived cases cannot transition out of archived — this state is
              final.
            </p>
          )}
        </form>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button type="submit" form="edit-case-form" disabled={submitting}>
            {submitting ? <Loader2 className="size-4 animate-spin" /> : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
