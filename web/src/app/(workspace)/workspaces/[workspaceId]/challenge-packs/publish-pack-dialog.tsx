"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  ValidateChallengePackResponse,
  PublishChallengePackResponse,
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
import { toast } from "sonner";
import { CheckCircle2, Loader2, Plus, XCircle } from "lucide-react";

interface PublishPackDialogProps {
  workspaceId: string;
}

export function PublishPackDialog({ workspaceId }: PublishPackDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [yaml, setYaml] = useState("");
  const [validating, setValidating] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [validationResult, setValidationResult] =
    useState<ValidateChallengePackResponse | null>(null);

  function reset() {
    setYaml("");
    setValidating(false);
    setPublishing(false);
    setValidationResult(null);
  }

  async function handleValidate() {
    if (!yaml.trim()) return;

    setValidating(true);
    setValidationResult(null);

    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result =
        await api.postRaw<ValidateChallengePackResponse>(
          `/v1/workspaces/${workspaceId}/challenge-packs/validate`,
          yaml,
          "application/yaml",
        );
      setValidationResult(result);
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        // 400 means validation failed — the body has the errors
        try {
          // ApiError doesn't carry the parsed body, so re-parse
          setValidationResult({
            valid: false,
            errors: [{ field: "", message: err.message }],
          });
        } catch {
          setValidationResult({
            valid: false,
            errors: [{ field: "", message: err.message }],
          });
        }
      } else {
        toast.error(
          err instanceof ApiError ? err.message : "Validation request failed",
        );
      }
    } finally {
      setValidating(false);
    }
  }

  async function handlePublish() {
    if (!yaml.trim()) return;

    setPublishing(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.postRaw<PublishChallengePackResponse>(
        `/v1/workspaces/${workspaceId}/challenge-packs`,
        yaml,
        "application/yaml",
      );
      toast.success("Challenge pack published");
      setOpen(false);
      reset();
      router.refresh();
      return result;
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to publish challenge pack");
      }
    } finally {
      setPublishing(false);
    }
  }

  const isValid = validationResult?.valid === true;
  const hasErrors =
    validationResult !== null &&
    !validationResult.valid &&
    validationResult.errors.length > 0;
  const busy = validating || publishing;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setOpen(v);
        if (!v) reset();
      }}
    >
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        Publish New Pack
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Publish Challenge Pack</DialogTitle>
          <DialogDescription>
            Paste a YAML bundle defining your challenge pack. Validate first,
            then publish.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              YAML Bundle
            </label>
            <textarea
              value={yaml}
              onChange={(e) => {
                setYaml(e.target.value);
                // Reset validation when content changes
                if (validationResult) setValidationResult(null);
              }}
              placeholder={`pack:\n  slug: my-pack\n  name: My Challenge Pack\n  description: ...\nversion: "1"\nchallenges:\n  - ...`}
              rows={14}
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm font-[family-name:var(--font-mono)] placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y"
            />
          </div>

          {/* Validation result */}
          {isValid && (
            <div className="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-400">
              <CheckCircle2 className="size-4 shrink-0" />
              Bundle is valid — ready to publish.
            </div>
          )}

          {hasErrors && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm">
              <div className="flex items-center gap-2 text-destructive mb-1.5">
                <XCircle className="size-4 shrink-0" />
                Validation failed
              </div>
              <ul className="space-y-1 text-xs text-destructive/80">
                {validationResult.errors.map((e, i) => (
                  <li key={i}>
                    {e.field ? (
                      <>
                        <code className="font-[family-name:var(--font-mono)]">
                          {e.field}
                        </code>
                        : {e.message}
                      </>
                    ) : (
                      e.message
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={busy}
          >
            Cancel
          </Button>
          <Button
            variant="outline"
            disabled={!yaml.trim() || busy}
            onClick={handleValidate}
          >
            {validating ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Validate"
            )}
          </Button>
          <Button disabled={!isValid || busy} onClick={handlePublish}>
            {publishing ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Publish"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
