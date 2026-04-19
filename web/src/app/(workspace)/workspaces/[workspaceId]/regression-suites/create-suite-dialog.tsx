"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ChallengePack,
  CreateRegressionSuiteInput,
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

interface CreateSuiteDialogProps {
  workspaceId: string;
  packs: ChallengePack[];
}

function hasRunnableVersion(pack: ChallengePack): boolean {
  return pack.versions.some((v) => v.lifecycle_status === "runnable");
}

export function CreateSuiteDialog({
  workspaceId,
  packs,
}: CreateSuiteDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);

  const eligiblePacks = packs.filter(hasRunnableVersion);
  const noEligible = eligiblePacks.length === 0;

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [packId, setPackId] = useState("");
  const [severity, setSeverity] = useState<RegressionSeverity>("warning");
  const [submitting, setSubmitting] = useState(false);

  function reset() {
    setName("");
    setDescription("");
    setPackId("");
    setSeverity("warning");
    setSubmitting(false);
  }

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;

    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error("Name is required");
      return;
    }
    if (!packId) {
      toast.error("Select a challenge pack");
      return;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const body: CreateRegressionSuiteInput = {
        source_challenge_pack_id: packId,
        name: trimmedName,
        description: description.trim() || undefined,
        default_gate_severity: severity,
      };
      await api.post<RegressionSuite>(
        `/v1/workspaces/${workspaceId}/regression-suites`,
        body,
      );
      toast.success("Regression suite created");
      setOpen(false);
      reset();
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to create regression suite");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const trigger = (
    <Button size="sm" disabled={noEligible}>
      <Plus data-icon="inline-start" className="size-4" />
      New Suite
    </Button>
  );

  return (
    <div className="inline-flex">
      <Dialog
        open={open}
        onOpenChange={(v) => {
          setOpen(v);
          if (!v) reset();
        }}
      >
        {noEligible ? (
          <span
            title="Publish an active challenge pack before creating a regression suite."
          >
            {trigger}
          </span>
        ) : (
          <DialogTrigger render={trigger} />
        )}
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New Regression Suite</DialogTitle>
            <DialogDescription>
              Suites hold a curated set of cases promoted from failures against
              a specific challenge pack.
            </DialogDescription>
          </DialogHeader>

          <form id="create-suite-form" onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                Name
              </label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Billing flow regressions"
                autoFocus
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
                placeholder="Optional — what does this suite protect?"
                rows={3}
                className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                Source Challenge Pack
              </label>
              <Select
                value={packId}
                onValueChange={(v) => v && setPackId(v)}
              >
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select a pack" />
                </SelectTrigger>
                <SelectContent>
                  {eligiblePacks.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground">
                Default Gate Severity
              </label>
              <Select
                value={severity}
                onValueChange={(v) =>
                  v && setSeverity(v as RegressionSeverity)
                }
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
              form="create-suite-form"
              disabled={submitting || !name.trim() || !packId}
            >
              {submitting ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "Create Suite"
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
