"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Pencil, Plus } from "lucide-react";

import {
  addDatasetExample,
  patchDatasetExample,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  DatasetExample,
  DatasetExampleStatus,
  PatchDatasetExampleInput,
  UpsertDatasetExampleInput,
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
import { JsonField } from "@/components/ui/json-field";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface ExampleFormDialogProps {
  workspaceId: string;
  datasetId: string;
  example?: DatasetExample;
  trigger?: "add" | "edit";
}

export function ExampleFormDialog({
  workspaceId,
  datasetId,
  example,
  trigger = example ? "edit" : "add",
}: ExampleFormDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const isEdit = trigger === "edit" && example != null;
  const [open, setOpen] = useState(false);
  const [externalId, setExternalId] = useState("");
  const [inputJson, setInputJson] = useState("{}");
  const [expectedJson, setExpectedJson] = useState("");
  const [tags, setTags] = useState("");
  const [status, setStatus] = useState<DatasetExampleStatus>("active");
  const [inputError, setInputError] = useState<string>();
  const [expectedError, setExpectedError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      if (example) {
        setExternalId(example.external_id ?? "");
        setInputJson(JSON.stringify(example.input, null, 2));
        setExpectedJson(
          example.expected != null
            ? JSON.stringify(example.expected, null, 2)
            : "",
        );
        setTags(example.tags.join(", "));
        setStatus(example.status);
      } else {
        setExternalId("");
        setInputJson("{}");
        setExpectedJson("");
        setTags("");
        setStatus("active");
      }
      setInputError(undefined);
      setExpectedError(undefined);
    }
  }, [open, example]);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting) return;

    let input: unknown;
    try {
      input = JSON.parse(inputJson);
    } catch {
      setInputError("Invalid JSON");
      return;
    }

    let expected: unknown | undefined;
    if (expectedJson.trim()) {
      try {
        expected = JSON.parse(expectedJson);
      } catch {
        setExpectedError("Invalid JSON");
        return;
      }
    }

    const tagList = tags
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);

      if (isEdit && example) {
        const patch: PatchDatasetExampleInput = {
          input,
          tags: tagList,
          status,
        };
        if (expected !== undefined) patch.expected = expected;
        await patchDatasetExample(
          api,
          workspaceId,
          datasetId,
          example.id,
          patch,
        );
        toast.success("Example updated");
      } else {
        const body: UpsertDatasetExampleInput = {
          input,
          tags: tagList.length > 0 ? tagList : undefined,
          status,
          source: "manual",
        };
        if (externalId.trim()) body.external_id = externalId.trim();
        if (expected !== undefined) body.expected = expected;
        await addDatasetExample(api, workspaceId, datasetId, body);
        toast.success("Example added");
      }

      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save example");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          isEdit ? (
            <Button variant="ghost" size="icon-xs" aria-label="Edit example" />
          ) : (
            <Button size="sm" />
          )
        }
      >
        {isEdit ? (
          <Pencil className="size-3.5" />
        ) : (
          <>
            <Plus data-icon="inline-start" className="size-4" />
            Add example
          </>
        )}
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {isEdit ? "Edit example" : "Add example"}
            </DialogTitle>
            <DialogDescription>
              {isEdit
                ? "Update input, expected output, tags, or status."
                : "Create a manual dataset example."}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
            {!isEdit && (
              <div>
                <label className="mb-1.5 block text-sm font-medium">
                  External ID{" "}
                  <span className="font-normal text-muted-foreground">
                    (optional)
                  </span>
                </label>
                <input
                  value={externalId}
                  onChange={(e) => setExternalId(e.target.value)}
                  className={inputClass}
                />
              </div>
            )}
            <JsonField
              label="Input"
              value={inputJson}
              onChange={setInputJson}
              error={inputError}
              rows={5}
            />
            <JsonField
              label="Expected"
              description="Optional expected output for scoring."
              value={expectedJson}
              onChange={setExpectedJson}
              error={expectedError}
              rows={4}
            />
            <div>
              <label className="mb-1.5 block text-sm font-medium">Tags</label>
              <input
                value={tags}
                onChange={(e) => setTags(e.target.value)}
                placeholder="comma-separated"
                className={inputClass}
              />
            </div>
            {isEdit && (
              <div>
                <label className="mb-1.5 block text-sm font-medium">Status</label>
                <select
                  value={status}
                  onChange={(e) =>
                    setStatus(e.target.value as DatasetExampleStatus)
                  }
                  className={inputClass}
                >
                  <option value="active">active</option>
                  <option value="archived">archived</option>
                  <option value="muted">muted</option>
                </select>
              </div>
            )}
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
              ) : isEdit ? (
                "Save"
              ) : (
                "Add"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
