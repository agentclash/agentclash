"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { useApiMutator } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
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
import { CheckCircle2, Loader2, Plus, XCircle, Maximize2, Minimize2 } from "lucide-react";
import Editor from "@monaco-editor/react";

interface PublishPackDialogProps {
  workspaceId: string;
}

export function PublishPackDialog({ workspaceId }: PublishPackDialogProps) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
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
    setIsFullscreen(false);
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
      await mutate(workspaceResourceKeys.challengePacks(workspaceId));
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
      <DialogContent
        className={
          isFullscreen
            ? "max-w-[100vw] w-[100vw] h-[100vh] m-0 rounded-none p-6 flex flex-col sm:max-w-none data-closed:zoom-out-95 data-open:zoom-in-95 data-closed:fade-out-0 data-open:fade-in-0"
            : "sm:max-w-3xl flex flex-col max-h-[90vh]"
        }
      >
        <Button
          variant="ghost"
          onClick={() => setIsFullscreen(!isFullscreen)}
          className="absolute top-2 right-10"
          size="icon-sm"
        >
          {isFullscreen ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
          <span className="sr-only">Toggle Fullscreen</span>
        </Button>

        <DialogHeader>
          <DialogTitle>Publish Challenge Pack</DialogTitle>
          <DialogDescription>
            Paste a YAML bundle defining your challenge pack. Validate first,
            then publish.
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 flex flex-col gap-3 py-2 min-h-[400px]">
          <div className="flex-1 border rounded-lg overflow-hidden relative">
            <Editor
              height="100%"
              defaultLanguage="yaml"
              theme="vs-dark"
              value={yaml}
              onChange={(value) => {
                setYaml(value || "");
                if (validationResult) setValidationResult(null);
              }}
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                wordWrap: "on",
                scrollBeyondLastLine: false,
                tabSize: 2,
                insertSpaces: true,
                formatOnPaste: true,
              }}
              loading={
                <div className="flex items-center justify-center h-full text-muted-foreground">
                  <Loader2 className="size-6 animate-spin" />
                </div>
              }
            />
          </div>

          {/* Validation result */}
          {isValid && (
            <div className="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-400 shrink-0">
              <CheckCircle2 className="size-4 shrink-0" />
              Bundle is valid — ready to publish.
            </div>
          )}

          {hasErrors && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm overflow-y-auto max-h-40 shrink-0">
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

        <DialogFooter className="shrink-0 mt-auto">
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
