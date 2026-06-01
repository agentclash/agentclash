"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { promoteDatasetTraceCandidate } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { DatasetTraceCandidate } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { JsonField } from "@/components/ui/json-field";
import {
  ExamplePayloadPreview,
  TagBadges,
} from "../dataset-ui-shared";

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface PromoteTraceDialogProps {
  workspaceId: string;
  datasetId: string;
  candidate: DatasetTraceCandidate;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPromoted?: () => void;
}

export function PromoteTraceDialog({
  workspaceId,
  datasetId,
  candidate,
  open,
  onOpenChange,
  onPromoted,
}: PromoteTraceDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [expectedJson, setExpectedJson] = useState("");
  const [expectedError, setExpectedError] = useState<string>();
  const [tags, setTags] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setExpectedJson(
      candidate.expected != null
        ? JSON.stringify(candidate.expected, null, 2)
        : candidate.output != null
          ? JSON.stringify(candidate.output, null, 2)
          : "",
    );
    setTags(candidate.tags.join(", "));
    setExpectedError(undefined);
  }, [candidate, open]);

  async function handlePromote() {
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
      .map((tag) => tag.trim())
      .filter(Boolean);

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await promoteDatasetTraceCandidate(
        api,
        workspaceId,
        datasetId,
        candidate.id,
        {
          expected,
          tags: tagList.length > 0 ? tagList : undefined,
        },
      );
      toast.success("Candidate promoted to example");
      onOpenChange(false);
      onPromoted?.();
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Promotion failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Promote trace candidate</DialogTitle>
          <DialogDescription>
            Review the captured trace and adjust expected output or tags before
            adding it to the dataset.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 max-h-[60vh] overflow-y-auto">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">
              {candidate.external_id ?? candidate.id.slice(0, 8)}
            </span>
            <TagBadges tags={candidate.tags} />
          </div>
          <ExamplePayloadPreview
            input={candidate.input}
            expected={candidate.output ?? candidate.expected}
            metadata={candidate.metadata}
          />
          <JsonField
            label="Expected output"
            description="Override the promoted expected value if needed."
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
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button onClick={handlePromote} disabled={submitting}>
            {submitting ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Promote"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
