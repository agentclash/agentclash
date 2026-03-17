"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuthStore } from "@/lib/stores/auth";
import { api } from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ArrowLeft, Loader2, Plus } from "lucide-react";

export default function CreateBuildPage() {
  const router = useRouter();
  const { activeWorkspaceId } = useAuthStore();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!activeWorkspaceId) return;

    setError("");
    if (!name.trim()) {
      setError("Name is required");
      return;
    }

    setSubmitting(true);
    try {
      const result = await api.createAgentBuild(activeWorkspaceId, {
        name: name.trim(),
        description: description.trim() || undefined,
      });
      router.push(`/builds/${result.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create build");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="max-w-3xl">
      <div className="mb-4">
        <Link
          href="/builds"
          className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
        >
          <ArrowLeft className="size-3" />
          Back to builds
        </Link>
      </div>

      <PageHeader
        eyebrow="Create"
        title="New Build"
        description="Define an agent build configuration"
      />

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-2">
          <Label className="text-xs text-text-2">Name</Label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g., my-llm-agent"
            className="max-w-md"
          />
        </div>

        <div className="space-y-2">
          <Label className="text-xs text-text-2">Description (optional)</Label>
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Brief description of this agent build"
            className="max-w-md"
          />
        </div>

        {error && (
          <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-3">
            <p className="text-xs text-status-fail">{error}</p>
          </div>
        )}

        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={submitting}>
            {submitting ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                Creating...
              </>
            ) : (
              <>
                <Plus className="size-3.5" data-icon="inline-start" />
                Create Build
              </>
            )}
          </Button>
          <Link href="/builds">
            <Button type="button" variant="outline">
              Cancel
            </Button>
          </Link>
        </div>
      </form>
    </div>
  );
}
