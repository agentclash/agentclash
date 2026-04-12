"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
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

const SECRET_KEY_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

interface UpsertSecretDialogProps {
  workspaceId: string;
}

export function UpsertSecretDialog({ workspaceId }: UpsertSecretDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [keyError, setKeyError] = useState("");

  function validateKey(k: string) {
    if (!k.trim()) {
      setKeyError("Key is required");
      return false;
    }
    if (!SECRET_KEY_PATTERN.test(k)) {
      setKeyError("Must match [A-Za-z_][A-Za-z0-9_]* (e.g. API_KEY, db_url)");
      return false;
    }
    if (k.length > 128) {
      setKeyError("Max 128 characters");
      return false;
    }
    setKeyError("");
    return true;
  }

  async function handleSubmit() {
    if (!validateKey(key) || !value.trim()) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.put(
        `/v1/workspaces/${workspaceId}/secrets/${encodeURIComponent(key.trim())}`,
        { value: value },
      );
      toast.success(`Secret "${key.trim()}" saved`);
      setOpen(false);
      setKey("");
      setValue("");
      setKeyError("");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to save secret");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New Secret
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Secret</DialogTitle>
          <DialogDescription>
            Store a secret value that agents can reference as {`\${secrets.KEY}`}.
            Values are encrypted at rest and never displayed after saving.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Key</label>
            <input
              type="text"
              value={key}
              onChange={(e) => {
                setKey(e.target.value);
                if (keyError) validateKey(e.target.value);
              }}
              placeholder="e.g. OPENAI_API_KEY"
              autoFocus
              className={`block w-full rounded-lg border bg-transparent px-3 py-2 text-sm font-[family-name:var(--font-mono)] placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 ${
                keyError ? "border-destructive" : "border-input focus:border-ring"
              }`}
            />
            {keyError && (
              <p className="mt-1 text-xs text-destructive">{keyError}</p>
            )}
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">Value</label>
            <textarea
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder="Secret value (encrypted at rest)"
              rows={3}
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm font-[family-name:var(--font-mono)] placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 resize-none"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={submitting}>
            Cancel
          </Button>
          <Button
            disabled={!key.trim() || !value.trim() || !!keyError || submitting}
            onClick={handleSubmit}
          >
            {submitting ? <Loader2 className="size-4 animate-spin" /> : "Save Secret"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
