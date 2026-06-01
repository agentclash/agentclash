"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Pencil } from "lucide-react";

import { patchDataset } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { Dataset, PatchDatasetInput } from "@/lib/api/types";
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
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface EditDatasetDialogProps {
  workspaceId: string;
  dataset: Dataset;
}

export function EditDatasetDialog({
  workspaceId,
  dataset,
}: EditDatasetDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(dataset.name);
  const [slug, setSlug] = useState(dataset.slug);
  const [description, setDescription] = useState(dataset.description);
  const [schemaEnforced, setSchemaEnforced] = useState(
    dataset.input_schema_enforced,
  );
  const [inputSchemaJson, setInputSchemaJson] = useState(
    dataset.input_schema ? JSON.stringify(dataset.input_schema, null, 2) : "",
  );
  const [schemaError, setSchemaError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setName(dataset.name);
      setSlug(dataset.slug);
      setDescription(dataset.description);
      setSchemaEnforced(dataset.input_schema_enforced);
      setInputSchemaJson(
        dataset.input_schema ? JSON.stringify(dataset.input_schema, null, 2) : "",
      );
      setSchemaError(undefined);
    }
  }, [open, dataset]);

  async function handleSave(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;

    const patch: PatchDatasetInput = {};
    const trimmedName = name.trim();
    const trimmedSlug = slug.trim();
    if (trimmedName !== dataset.name) patch.name = trimmedName;
    if (trimmedSlug !== dataset.slug) patch.slug = trimmedSlug;
    if (description !== dataset.description) patch.description = description;
    if (schemaEnforced !== dataset.input_schema_enforced) {
      patch.input_schema_enforced = schemaEnforced;
    }

    if (inputSchemaJson.trim()) {
      try {
        patch.input_schema = JSON.parse(inputSchemaJson);
      } catch {
        setSchemaError("Invalid JSON");
        return;
      }
    } else if (dataset.input_schema) {
      patch.input_schema = {};
    }

    if (Object.keys(patch).length === 0) {
      setOpen(false);
      return;
    }
    if (!trimmedName || !trimmedSlug) {
      toast.error("Name and slug are required");
      return;
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await patchDataset(api, workspaceId, dataset.id, patch);
      toast.success("Dataset updated");
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update dataset");
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
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSave}>
          <DialogHeader>
            <DialogTitle>Edit dataset</DialogTitle>
            <DialogDescription>
              Update dataset metadata and optional input schema enforcement.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <label className="mb-1.5 block text-sm font-medium">Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className={inputClass}
                required
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">Slug</label>
              <input
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
                className={inputClass}
                required
              />
            </div>
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Description
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={2}
                className={`${inputClass} resize-y`}
              />
            </div>
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
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "Save"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
