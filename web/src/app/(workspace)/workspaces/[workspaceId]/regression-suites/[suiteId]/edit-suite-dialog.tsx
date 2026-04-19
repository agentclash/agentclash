"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Pencil } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  PatchRegressionSuiteInput,
  RegressionSeverity,
  RegressionSuite,
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

const SEVERITY_OPTIONS: RegressionSeverity[] = ["info", "warning", "blocking"];

interface EditSuiteDialogProps {
  workspaceId: string;
  suite: RegressionSuite;
}

export function EditSuiteDialog({
  workspaceId,
  suite,
}: EditSuiteDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(suite.name);
  const [description, setDescription] = useState(suite.description);
  const [severity, setSeverity] = useState<RegressionSeverity>(
    suite.default_gate_severity,
  );
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setName(suite.name);
      setDescription(suite.description);
      setSeverity(suite.default_gate_severity);
    }
  }, [open, suite]);

  async function handleSave(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;

    const patch: PatchRegressionSuiteInput = {};
    const trimmedName = name.trim();
    if (trimmedName !== suite.name) patch.name = trimmedName;
    if (description !== suite.description) patch.description = description;
    if (severity !== suite.default_gate_severity) {
      patch.default_gate_severity = severity;
    }

    if (Object.keys(patch).length === 0) {
      setOpen(false);
      return;
    }
    if (patch.name !== undefined && !patch.name) {
      toast.error("Name cannot be empty");
      return;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.patch<RegressionSuite>(
        `/v1/workspaces/${workspaceId}/regression-suites/${suite.id}`,
        patch,
      );
      toast.success("Suite updated");
      setOpen(false);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          toast.error(
            "You don't have permission to edit this suite.",
          );
        } else {
          toast.error(err.message);
        }
      } else {
        toast.error("Failed to update suite");
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
          <DialogTitle>Edit Suite</DialogTitle>
          <DialogDescription>
            Update the suite name, description, and default gate severity.
          </DialogDescription>
        </DialogHeader>

        <form
          id="edit-suite-form"
          onSubmit={handleSave}
          className="space-y-4"
        >
          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Name
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
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

          <div className="space-y-1.5">
            <label className="text-xs font-medium text-muted-foreground">
              Default Gate Severity
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
        </form>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            form="edit-suite-form"
            disabled={submitting}
          >
            {submitting ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Save"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
