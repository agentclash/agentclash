"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

import {
  createWorkspaceAgentTryout,
  listAgentTryoutTemplates,
} from "@/lib/api/agent-tryouts";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentTryoutTemplate } from "@/lib/api/types";
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

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

export function LaunchTryoutDialog({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [templates, setTemplates] = useState<AgentTryoutTemplate[]>([]);
  const [templateSlug, setTemplateSlug] = useState("");
  const [fieldValues, setFieldValues] = useState<Record<string, string>>({});

  const loadTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const res = await listAgentTryoutTemplates(api);
      const available = res.items.filter((template) => template.available);
      setTemplates(available);
      setTemplateSlug((current) => current || (available[0]?.slug ?? ""));
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load tryout templates",
      );
    } finally {
      setLoading(false);
    }
  }, [getAccessToken]);

  useEffect(() => {
    if (open) {
      void loadTemplates();
    }
  }, [open, loadTemplates]);

  const template = useMemo(
    () => templates.find((item) => item.slug === templateSlug),
    [templates, templateSlug],
  );
  const properties = template?.input_schema.properties ?? {};
  const required = useMemo(
    () => new Set(template?.input_schema.required ?? []),
    [template],
  );

  useEffect(() => {
    setFieldValues({});
  }, [templateSlug]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitting || !template) return;

    const input: Record<string, unknown> = {};
    for (const [field, spec] of Object.entries(properties)) {
      const raw = (fieldValues[field] ?? "").trim();
      if (!raw) {
        if (required.has(field)) {
          toast.error(`${fieldLabel(field)} is required`);
          return;
        }
        continue;
      }
      if (spec.type === "integer" || spec.type === "number") {
        const value = Number(raw);
        if (!Number.isFinite(value)) {
          toast.error(`${fieldLabel(field)} must be a number`);
          return;
        }
        if (spec.minimum !== undefined && value < spec.minimum) {
          toast.error(`${fieldLabel(field)} must be at least ${spec.minimum}`);
          return;
        }
        if (spec.maximum !== undefined && value > spec.maximum) {
          toast.error(`${fieldLabel(field)} must be at most ${spec.maximum}`);
          return;
        }
        input[field] = spec.type === "integer" ? Math.trunc(value) : value;
      } else {
        input[field] = raw;
      }
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const tryout = await createWorkspaceAgentTryout(api, workspaceId, {
        template_slug: template.slug,
        input,
      });
      toast.success("Tryout launched");
      setOpen(false);
      await mutateMany([workspaceResourceKeys.agentTryouts(workspaceId)]);
      router.push(`/workspaces/${workspaceId}/agent-tryouts/${tryout.id}`);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to launch tryout",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New tryout
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>New agent tryout</DialogTitle>
            <DialogDescription>
              Pick a task, paste your real input, and watch an agent do the
              work. Runs in an isolated sandbox on your workspace provider keys.
            </DialogDescription>
          </DialogHeader>
          {loading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
              <div>
                <label className="mb-1.5 block text-sm font-medium">Task</label>
                <select
                  value={templateSlug}
                  onChange={(e) => setTemplateSlug(e.target.value)}
                  className={inputClass}
                >
                  {templates.map((item) => (
                    <option key={item.slug} value={item.slug}>
                      {item.name}
                    </option>
                  ))}
                </select>
                {template ? (
                  <p className="mt-1.5 text-xs text-muted-foreground">
                    {template.description} Up to{" "}
                    {Math.round(template.max_duration_seconds / 60)} min, capped
                    at ${template.max_cost_usd.toFixed(2)}.
                  </p>
                ) : null}
              </div>
              {Object.entries(properties).map(([field, spec]) => (
                <div key={field}>
                  <label className="mb-1.5 block text-sm font-medium">
                    {fieldLabel(field)}
                    {required.has(field) ? null : (
                      <span className="font-normal text-muted-foreground">
                        {" "}
                        (optional)
                      </span>
                    )}
                  </label>
                  {spec.type === "string" ? (
                    <textarea
                      value={fieldValues[field] ?? ""}
                      onChange={(e) =>
                        setFieldValues((current) => ({
                          ...current,
                          [field]: e.target.value,
                        }))
                      }
                      rows={required.has(field) ? 5 : 2}
                      className={`${inputClass} resize-y`}
                    />
                  ) : (
                    <input
                      type="number"
                      value={fieldValues[field] ?? ""}
                      min={spec.minimum}
                      max={spec.maximum}
                      onChange={(e) =>
                        setFieldValues((current) => ({
                          ...current,
                          [field]: e.target.value,
                        }))
                      }
                      className={inputClass}
                    />
                  )}
                </div>
              ))}
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
            <Button type="submit" disabled={submitting || loading || !template}>
              {submitting ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                "Launch tryout"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function fieldLabel(field: string): string {
  const spaced = field.replaceAll("_", " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}
