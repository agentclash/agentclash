"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

import { createDataset } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { ChallengePack, ChallengePackVersion } from "@/lib/api/types";
import { useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
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
import { JsonField } from "@/components/ui/json-field";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

export function CreateDatasetDialog({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [schemaEnforced, setSchemaEnforced] = useState(false);
  const [inputSchemaJson, setInputSchemaJson] = useState("");
  const [schemaError, setSchemaError] = useState<string>();
  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [packId, setPackId] = useState("");
  const [packVersions, setPackVersions] = useState<ChallengePackVersion[]>([]);
  const [packVersionId, setPackVersionId] = useState("");

  const loadPacks = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.get<{ items: ChallengePack[] }>(
        `/v1/workspaces/${workspaceId}/challenge-packs`,
      );
      setPacks(res.items);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load challenge packs",
      );
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) {
      void loadPacks();
    }
  }, [open, loadPacks]);

  useEffect(() => {
    if (!packId) {
      setPackVersions([]);
      setPackVersionId("");
      return;
    }
    const pack = packs.find((item) => item.id === packId);
    const runnable = (pack?.versions ?? []).filter(
      (version) => version.lifecycle_status === "runnable",
    );
    setPackVersions(runnable);
    setPackVersionId(runnable[runnable.length - 1]?.id ?? "");
  }, [packId, packs]);

  function reset() {
    setSlug("");
    setName("");
    setDescription("");
    setSchemaEnforced(false);
    setInputSchemaJson("");
    setSchemaError(undefined);
    setPackId("");
    setPackVersionId("");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitting) return;
    if (!slug.trim() || !name.trim()) {
      toast.error("Slug and name are required");
      return;
    }

    let input_schema: Record<string, unknown> | undefined;
    if (inputSchemaJson.trim()) {
      try {
        input_schema = JSON.parse(inputSchemaJson) as Record<string, unknown>;
      } catch {
        setSchemaError("Invalid JSON");
        return;
      }
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await createDataset(api, workspaceId, {
        slug: slug.trim(),
        name: name.trim(),
        description: description.trim() || undefined,
        input_schema,
        input_schema_enforced: schemaEnforced,
        default_challenge_pack_version_id: packVersionId || undefined,
      });
      toast.success("Dataset created");
      setOpen(false);
      reset();
      await mutateMany([workspaceResourceKeys.datasets(workspaceId)]);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create dataset");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) reset();
      }}
    >
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New dataset
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>New dataset</DialogTitle>
            <DialogDescription>
              Create a workspace dataset for eval examples and CI baselines.
            </DialogDescription>
          </DialogHeader>
          {loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
              <TextField label="Slug" value={slug} onChange={setSlug} required />
              <TextField label="Name" value={name} onChange={setName} required />
              <TextField
                label="Description"
                value={description}
                onChange={setDescription}
                optional
                multiline
              />
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={schemaEnforced}
                  onChange={(e) => setSchemaEnforced(e.target.checked)}
                />
                Enforce input schema on examples
              </label>
              <JsonField
                label="Input schema (JSON)"
                value={inputSchemaJson}
                onChange={setInputSchemaJson}
                error={schemaError}
                rows={4}
              />
              <div>
                <label className="mb-1.5 block text-sm font-medium">
                  Default challenge pack{" "}
                  <span className="font-normal text-muted-foreground">
                    (optional)
                  </span>
                </label>
                <select
                  value={packId}
                  onChange={(e) => setPackId(e.target.value)}
                  className={inputClass}
                >
                  <option value="">None</option>
                  {packs.map((pack) => (
                    <option key={pack.id} value={pack.id}>
                      {pack.name}
                    </option>
                  ))}
                </select>
              </div>
              {packId ? (
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Default pack version
                  </label>
                  <select
                    value={packVersionId}
                    onChange={(e) => setPackVersionId(e.target.value)}
                    className={inputClass}
                  >
                    <option value="">Select version...</option>
                    {packVersions.map((version) => (
                      <option key={version.id} value={version.id}>
                        v{version.version_number}
                      </option>
                    ))}
                  </select>
                </div>
              ) : null}
            </div>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitting || loading}>
              {submitting ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "Create"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function TextField({
  label,
  value,
  onChange,
  required,
  optional,
  multiline,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  optional?: boolean;
  multiline?: boolean;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">
        {label}
        {optional ? (
          <span className="font-normal text-muted-foreground"> (optional)</span>
        ) : null}
      </label>
      {multiline ? (
        <textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          rows={2}
          className={`${inputClass} resize-y`}
        />
      ) : (
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          required={required}
          className={inputClass}
        />
      )}
    </div>
  );
}
