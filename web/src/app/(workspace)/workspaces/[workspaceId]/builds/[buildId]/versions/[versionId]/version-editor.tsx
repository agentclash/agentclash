"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentBuildVersion,
  ValidationResult,
} from "@/lib/api/types";
import { AGENT_KINDS } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { JsonField } from "@/components/ui/json-field";
import { toast } from "sonner";
import { Loader2, Save, ShieldCheck, CheckCircle } from "lucide-react";

interface VersionEditorProps {
  version: AgentBuildVersion;
}

function jsonStr(val: unknown): string {
  if (val === null || val === undefined) return "";
  if (typeof val === "string") return val;
  return JSON.stringify(val, null, 2);
}

const specFields = [
  { key: "policy_spec", label: "Policy Spec", description: "Must contain an \"instructions\" field.", required: true as const },
  { key: "interface_spec", label: "Interface Spec", description: undefined, required: true as const },
  { key: "model_spec", label: "Model Spec", description: undefined, required: false as const },
  { key: "reasoning_spec", label: "Reasoning Spec", description: undefined, required: false as const },
  { key: "memory_spec", label: "Memory Spec", description: undefined, required: false as const },
  { key: "workflow_spec", label: "Workflow Spec", description: undefined, required: false as const },
  { key: "guardrail_spec", label: "Guardrail Spec", description: undefined, required: false as const },
  { key: "output_schema", label: "Output Schema", description: undefined, required: false as const },
  { key: "trace_contract", label: "Trace Contract", description: undefined, required: false as const },
  { key: "publication_spec", label: "Publication Spec", description: undefined, required: false as const },
] as const;

type SpecKey = (typeof specFields)[number]["key"];

export function VersionEditor({
  version,
}: VersionEditorProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const isLocked = version.version_status === "ready";

  const [agentKind, setAgentKind] = useState(version.agent_kind);
  const [specs, setSpecs] = useState<Record<SpecKey, string>>(() => {
    const initial: Record<string, string> = {};
    for (const f of specFields) {
      initial[f.key] = jsonStr(
        version[f.key as keyof AgentBuildVersion],
      );
    }
    return initial as Record<SpecKey, string>;
  });

  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);
  const [markingReady, setMarkingReady] = useState(false);
  const [validationErrors, setValidationErrors] = useState<
    Record<string, string>
  >({});

  function updateSpec(key: SpecKey, value: string) {
    setSpecs((prev) => ({ ...prev, [key]: value }));
    // Clear validation error for this field when edited
    if (validationErrors[key]) {
      setValidationErrors((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    }
  }

  function buildRequestBody() {
    const body: Record<string, unknown> = { agent_kind: agentKind };
    for (const f of specFields) {
      const raw = specs[f.key].trim();
      if (raw) {
        try {
          body[f.key] = JSON.parse(raw);
        } catch {
          toast.error(`Invalid JSON in ${f.label}`);
          return null;
        }
      } else if (f.required) {
        body[f.key] = {};
      }
    }
    return body;
  }

  async function handleSave() {
    const body = buildRequestBody();
    if (!body) return;

    setSaving(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.patch(`/v1/agent-build-versions/${version.id}`, body);
      toast.success("Saved");
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  async function handleValidate() {
    setValidating(true);
    setValidationErrors({});
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await api.post<ValidationResult>(
        `/v1/agent-build-versions/${version.id}/validate`,
      );
      if (result.valid) {
        toast.success("Validation passed");
      } else {
        const errorMap: Record<string, string> = {};
        for (const e of result.errors) {
          errorMap[e.field] = e.message;
        }
        setValidationErrors(errorMap);
        toast.error(`${result.errors.length} validation error(s)`);
      }
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Validation failed",
      );
    } finally {
      setValidating(false);
    }
  }

  async function handleMarkReady() {
    setMarkingReady(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await api.post(`/v1/agent-build-versions/${version.id}/ready`);
      toast.success("Version marked as ready");
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.code === "validation_failed") {
        toast.error("Cannot mark ready — fix validation errors first");
        handleValidate();
      } else {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to mark ready",
        );
      }
    } finally {
      setMarkingReady(false);
    }
  }

  return (
    <div className="max-w-3xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-sm font-semibold">
            Version {version.version_number}
          </h2>
          <Badge
            variant={
              version.version_status === "ready"
                ? "default"
                : version.version_status === "draft"
                  ? "outline"
                  : "secondary"
            }
          >
            {version.version_status}
          </Badge>
        </div>
        {!isLocked && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleValidate}
              disabled={validating}
            >
              {validating ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <ShieldCheck data-icon="inline-start" className="size-4" />
                  Validate
                </>
              )}
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleSave}
              disabled={saving}
            >
              {saving ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <Save data-icon="inline-start" className="size-4" />
                  Save Draft
                </>
              )}
            </Button>
            <Button size="sm" onClick={handleMarkReady} disabled={markingReady}>
              {markingReady ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <>
                  <CheckCircle data-icon="inline-start" className="size-4" />
                  Mark Ready
                </>
              )}
            </Button>
          </div>
        )}
      </div>

      {isLocked && (
        <div className="mb-6 rounded-lg border border-white/[0.06] bg-white/[0.02] p-3 text-sm text-muted-foreground">
          This version is locked. Fields cannot be edited after marking as ready.
        </div>
      )}

      {/* Agent Kind */}
      <div className="mb-6">
        <label className="mb-1.5 block text-sm font-medium">Agent Kind</label>
        <select
          value={agentKind}
          onChange={(e) => setAgentKind(e.target.value)}
          disabled={isLocked}
          className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {AGENT_KINDS.map((kind) => (
            <option key={kind} value={kind}>
              {kind}
            </option>
          ))}
        </select>
        {validationErrors.agent_kind && (
          <p className="mt-1 text-xs text-destructive">
            {validationErrors.agent_kind}
          </p>
        )}
      </div>

      {/* Spec fields */}
      <div className="space-y-5">
        {specFields.map((f) => (
          <JsonField
            key={f.key}
            label={
              f.label +
              (f.required ? "" : " (optional)")
            }
            description={f.description}
            value={specs[f.key]}
            onChange={(v) => updateSpec(f.key, v)}
            error={validationErrors[f.key]}
            disabled={isLocked}
          />
        ))}
      </div>
    </div>
  );
}
