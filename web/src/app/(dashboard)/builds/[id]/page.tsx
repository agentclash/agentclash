"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useAuthStore } from "@/lib/stores/auth";
import {
  api,
  type AgentBuildResponse,
  type AgentBuildVersionResponse,
  type AgentBuildToolBinding,
  type AgentBuildKnowledgeSourceBinding,
  type ValidationError,
} from "@/lib/api/client";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ArrowLeft,
  Loader2,
  Save,
  Shield,
  Rocket,
  Check,
  AlertCircle,
  ChevronDown,
  ChevronRight,
  Plus,
} from "lucide-react";

type BuildWithVersions = AgentBuildResponse & { versions: AgentBuildVersionResponse[] };

const AGENT_KINDS = [
  "llm_agent",
  "workflow_agent",
  "programmatic_agent",
  "multi_agent_system",
  "hosted_external",
] as const;

type VersionStatusVariant = "pass" | "fail" | "warn" | "pending" | "neutral";

function getVersionStatusVariant(status: string): VersionStatusVariant {
  switch (status) {
    case "ready": return "pass";
    case "draft": return "pending";
    case "invalid": return "fail";
    default: return "neutral";
  }
}

const variantStyles: Record<VersionStatusVariant, string> = {
  pass: "text-status-pass bg-status-pass/10",
  fail: "text-status-fail bg-status-fail/10",
  warn: "text-status-warn bg-status-warn/10",
  pending: "text-text-3 bg-surface",
  neutral: "text-text-2 bg-surface",
};

function VersionStatusBadge({ status }: { status: string }) {
  const variant = getVersionStatusVariant(status);
  return (
    <span
      className={`
        inline-flex items-center
        font-[family-name:var(--font-mono)] text-[11px] font-semibold
        uppercase tracking-[0.06em]
        px-2.5 py-1 rounded
        ${variantStyles[variant]}
      `}
    >
      {status}
    </span>
  );
}

function CollapsibleSection({
  title,
  defaultOpen = false,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <Card className="bg-card">
      <CardHeader
        className="cursor-pointer select-none"
        onClick={() => setOpen((v) => !v)}
      >
        <CardTitle className="flex items-center gap-2 text-sm">
          {open ? (
            <ChevronDown className="size-4 text-text-3" />
          ) : (
            <ChevronRight className="size-4 text-text-3" />
          )}
          {title}
        </CardTitle>
      </CardHeader>
      {open && <CardContent>{children}</CardContent>}
    </Card>
  );
}

function JsonTextarea({
  value,
  onChange,
}: {
  value: Record<string, unknown> | unknown[];
  onChange: (v: Record<string, unknown> | unknown[]) => void;
}) {
  const [text, setText] = useState(JSON.stringify(value, null, 2));
  const [parseError, setParseError] = useState(false);

  useEffect(() => {
    setText(JSON.stringify(value, null, 2));
  }, [value]);

  return (
    <div>
      <textarea
        value={text}
        onChange={(e) => {
          setText(e.target.value);
          try {
            const parsed = JSON.parse(e.target.value);
            onChange(parsed);
            setParseError(false);
          } catch {
            setParseError(true);
          }
        }}
        className={`w-full min-h-[120px] rounded-lg border p-3 font-[family-name:var(--font-mono)] text-xs text-text-1 focus:outline-none focus:ring-1 focus:ring-ds-accent bg-surface ${parseError ? "border-status-fail" : "border-border"}`}
      />
      {parseError && (
        <p className="text-[10px] text-status-fail mt-1">Invalid JSON</p>
      )}
    </div>
  );
}

export default function BuildDetailPage() {
  const params = useParams();
  const buildId = params.id as string;
  const { activeWorkspaceId } = useAuthStore();

  const [build, setBuild] = useState<BuildWithVersions | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [activeVersion, setActiveVersion] = useState<AgentBuildVersionResponse | null>(null);
  const [agentKind, setAgentKind] = useState("llm_agent");
  const [instructions, setInstructions] = useState("");
  const [interfaceSpec, setInterfaceSpec] = useState<Record<string, unknown>>({});
  const [outputSchema, setOutputSchema] = useState<Record<string, unknown>>({});
  const [tools, setTools] = useState<AgentBuildToolBinding[]>([]);
  const [knowledgeSources, setKnowledgeSources] = useState<AgentBuildKnowledgeSourceBinding[]>([]);
  const [guardrailSpec, setGuardrailSpec] = useState<Record<string, unknown>>({});
  const [reasoningSpec, setReasoningSpec] = useState<Record<string, unknown>>({});
  const [memorySpec, setMemorySpec] = useState<Record<string, unknown>>({});
  const [workflowSpec, setWorkflowSpec] = useState<Record<string, unknown>>({});
  const [modelSpec, setModelSpec] = useState<Record<string, unknown>>({});
  const [publicationSpec, setPublicationSpec] = useState<Record<string, unknown>>({});

  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);
  const [validationErrors, setValidationErrors] = useState<ValidationError[]>([]);
  const [validationSuccess, setValidationSuccess] = useState(false);
  const [markingReady, setMarkingReady] = useState(false);
  const [actionError, setActionError] = useState("");
  const [actionSuccess, setActionSuccess] = useState("");

  const [deployName, setDeployName] = useState("");
  const [runtimeProfileId, setRuntimeProfileId] = useState("");
  const [providerAccountId, setProviderAccountId] = useState("");
  const [modelAliasId, setModelAliasId] = useState("");
  const [deploying, setDeploying] = useState(false);

  const loadBuild = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const result = await api.getAgentBuild(buildId);
      setBuild(result);
      if (result.versions && result.versions.length > 0) {
        populateVersionForm(result.versions[0]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load build");
    } finally {
      setLoading(false);
    }
  }, [buildId]);

  function populateVersionForm(v: AgentBuildVersionResponse) {
    setActiveVersion(v);
    setAgentKind(v.agent_kind || "llm_agent");
    setInstructions(
      (v.policy_spec as Record<string, unknown>)?.instructions as string || ""
    );
    setInterfaceSpec(v.interface_spec || {});
    setOutputSchema(v.output_schema || {});
    setTools(v.tools || []);
    setKnowledgeSources(v.knowledge_sources || []);
    setGuardrailSpec(v.guardrail_spec || {});
    setReasoningSpec(v.reasoning_spec || {});
    setMemorySpec(v.memory_spec || {});
    setWorkflowSpec(v.workflow_spec || {});
    setModelSpec(v.model_spec || {});
    setPublicationSpec(v.publication_spec || {});
  }

  useEffect(() => {
    loadBuild();
  }, [loadBuild]);

  function buildVersionPayload() {
    return {
      agent_kind: agentKind,
      policy_spec: instructions ? { instructions } : {},
      interface_spec: interfaceSpec,
      output_schema: outputSchema,
      tools,
      knowledge_sources: knowledgeSources,
      guardrail_spec: guardrailSpec,
      reasoning_spec: reasoningSpec,
      memory_spec: memorySpec,
      workflow_spec: workflowSpec,
      model_spec: modelSpec,
      publication_spec: publicationSpec,
    };
  }

  async function handleSave() {
    setSaving(true);
    setActionError("");
    setActionSuccess("");
    try {
      const payload = buildVersionPayload();
      if (activeVersion) {
        const updated = await api.updateAgentBuildVersion(activeVersion.id, payload);
        setActiveVersion(updated);
        setActionSuccess("Draft saved");
      } else {
        const created = await api.createAgentBuildVersion(buildId, payload);
        setActiveVersion(created);
        setActionSuccess("Version created");
      }
      await loadBuild();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  }

  async function handleValidate() {
    if (!activeVersion) return;
    setValidating(true);
    setValidationErrors([]);
    setValidationSuccess(false);
    setActionError("");
    setActionSuccess("");
    try {
      const result = await api.validateAgentBuildVersion(activeVersion.id);
      if (result.valid) {
        setValidationSuccess(true);
        setActionSuccess("Validation passed");
      } else {
        setValidationErrors(result.errors);
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Validation failed");
    } finally {
      setValidating(false);
    }
  }

  async function handleMarkReady() {
    if (!activeVersion) return;
    setMarkingReady(true);
    setActionError("");
    setActionSuccess("");
    try {
      const updated = await api.markAgentBuildVersionReady(activeVersion.id);
      setActiveVersion(updated);
      setActionSuccess("Version marked as ready");
      await loadBuild();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to mark ready");
    } finally {
      setMarkingReady(false);
    }
  }

  async function handleCreateNewVersion() {
    setSaving(true);
    setActionError("");
    setActionSuccess("");
    try {
      const payload = buildVersionPayload();
      const created = await api.createAgentBuildVersion(buildId, payload);
      populateVersionForm(created);
      setActionSuccess("New version created");
      await loadBuild();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create version");
    } finally {
      setSaving(false);
    }
  }

  async function handleDeploy(e: React.FormEvent) {
    e.preventDefault();
    if (!activeWorkspaceId || !activeVersion || !build) return;
    setDeploying(true);
    setActionError("");
    setActionSuccess("");
    try {
      await api.createAgentDeployment(activeWorkspaceId, {
        name: deployName.trim(),
        agent_build_id: build.id,
        build_version_id: activeVersion.id,
        runtime_profile_id: runtimeProfileId.trim(),
        provider_account_id: providerAccountId.trim() || undefined,
        model_alias_id: modelAliasId.trim() || undefined,
      });
      setActionSuccess("Deployment created");
      setDeployName("");
      setRuntimeProfileId("");
      setProviderAccountId("");
      setModelAliasId("");
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create deployment");
    } finally {
      setDeploying(false);
    }
  }

  if (loading) {
    return (
      <div className="max-w-4xl space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl">
        <div className="mb-4">
          <Link
            href="/builds"
            className="inline-flex items-center gap-1.5 text-xs text-text-3 hover:text-text-1 transition-colors"
          >
            <ArrowLeft className="size-3" />
            Back to builds
          </Link>
        </div>
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-4">
          <p className="text-sm text-status-fail">{error}</p>
        </div>
      </div>
    );
  }

  if (!build) return null;

  const isDraft = activeVersion?.version_status === "draft";
  const isReady = activeVersion?.version_status === "ready";

  return (
    <div className="max-w-4xl">
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
        eyebrow="Build"
        title={build.name}
        description={build.description || `Build ${build.id.slice(0, 8)}`}
      />

      {actionError && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-3 mb-4">
          <p className="text-xs text-status-fail">{actionError}</p>
        </div>
      )}

      {actionSuccess && (
        <div className="rounded-lg border border-status-pass/20 bg-status-pass/5 p-3 mb-4">
          <p className="text-xs text-status-pass flex items-center gap-1.5">
            <Check className="size-3" />
            {actionSuccess}
          </p>
        </div>
      )}

      {validationErrors.length > 0 && (
        <div className="rounded-lg border border-status-fail/20 bg-status-fail/5 p-3 mb-4">
          <p className="text-xs font-medium text-status-fail mb-2 flex items-center gap-1.5">
            <AlertCircle className="size-3" />
            Validation errors
          </p>
          <ul className="space-y-1">
            {validationErrors.map((ve, i) => (
              <li key={i} className="text-[11px] text-status-fail">
                <span className="font-[family-name:var(--font-mono)]">{ve.field}</span>: {ve.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      {build.versions && build.versions.length > 1 && (
        <div className="mb-6">
          <Label className="text-xs text-text-2 mb-2 block">Versions</Label>
          <div className="flex flex-wrap gap-2">
            {build.versions.map((v) => (
              <button
                key={v.id}
                type="button"
                onClick={() => populateVersionForm(v)}
                className={`
                  inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs
                  font-[family-name:var(--font-mono)] border cursor-pointer transition-colors
                  ${activeVersion?.id === v.id
                    ? "border-ds-accent text-ds-accent bg-ds-accent/5"
                    : "border-border text-text-2 hover:border-text-3"
                  }
                `}
              >
                v{v.version_number}
                <VersionStatusBadge status={v.version_status} />
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-4">
        <CollapsibleSection title="Basics" defaultOpen>
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              {activeVersion && (
                <div className="flex items-center gap-2">
                  <span className="text-xs text-text-3">Status:</span>
                  <VersionStatusBadge status={activeVersion.version_status} />
                </div>
              )}
              {activeVersion && (
                <div className="text-xs text-text-3">
                  Version {activeVersion.version_number}
                </div>
              )}
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-text-2">Agent Kind</Label>
              <select
                value={agentKind}
                onChange={(e) => setAgentKind(e.target.value)}
                className="w-full max-w-md rounded-lg border border-border bg-surface p-2 text-sm text-text-1 font-[family-name:var(--font-mono)] focus:outline-none focus:ring-1 focus:ring-ds-accent"
              >
                {AGENT_KINDS.map((kind) => (
                  <option key={kind} value={kind}>
                    {kind}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Instructions" defaultOpen>
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Policy Instructions</Label>
            <textarea
              value={instructions}
              onChange={(e) => setInstructions(e.target.value)}
              placeholder="Enter agent instructions..."
              className="w-full min-h-[160px] rounded-lg border border-border bg-surface p-3 text-sm text-text-1 focus:outline-none focus:ring-1 focus:ring-ds-accent"
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Interface">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Interface Spec (JSON)</Label>
            <JsonTextarea
              value={interfaceSpec}
              onChange={(v) => setInterfaceSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Output Schema">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Output Schema (JSON)</Label>
            <JsonTextarea
              value={outputSchema}
              onChange={(v) => setOutputSchema(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Tools">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Tools (JSON Array)</Label>
            <JsonTextarea
              value={tools}
              onChange={(v) => setTools(v as AgentBuildToolBinding[])}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Knowledge Sources">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Knowledge Sources (JSON Array)</Label>
            <JsonTextarea
              value={knowledgeSources}
              onChange={(v) => setKnowledgeSources(v as AgentBuildKnowledgeSourceBinding[])}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Guardrails">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Guardrail Spec (JSON)</Label>
            <JsonTextarea
              value={guardrailSpec}
              onChange={(v) => setGuardrailSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Reasoning">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Reasoning Spec (JSON)</Label>
            <JsonTextarea
              value={reasoningSpec}
              onChange={(v) => setReasoningSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Memory">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Memory Spec (JSON)</Label>
            <JsonTextarea
              value={memorySpec}
              onChange={(v) => setMemorySpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Workflow">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Workflow Spec (JSON)</Label>
            <JsonTextarea
              value={workflowSpec}
              onChange={(v) => setWorkflowSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Model">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Model Spec (JSON)</Label>
            <JsonTextarea
              value={modelSpec}
              onChange={(v) => setModelSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Publication">
          <div className="space-y-2">
            <Label className="text-xs text-text-2">Publication Spec (JSON)</Label>
            <JsonTextarea
              value={publicationSpec}
              onChange={(v) => setPublicationSpec(v as Record<string, unknown>)}
            />
          </div>
        </CollapsibleSection>
      </div>

      <div className="flex flex-wrap items-center gap-3 mt-8">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? (
            <>
              <Loader2 className="size-3.5 animate-spin" />
              Saving...
            </>
          ) : (
            <>
              <Save className="size-3.5" />
              {activeVersion ? "Save Draft" : "Create Version"}
            </>
          )}
        </Button>

        {activeVersion && (
          <Button variant="outline" onClick={handleValidate} disabled={validating}>
            {validating ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                Validating...
              </>
            ) : (
              <>
                <Shield className="size-3.5" />
                Validate
              </>
            )}
          </Button>
        )}

        {activeVersion && isDraft && (
          <Button variant="outline" onClick={handleMarkReady} disabled={markingReady}>
            {markingReady ? (
              <>
                <Loader2 className="size-3.5 animate-spin" />
                Marking...
              </>
            ) : (
              <>
                <Check className="size-3.5" />
                Mark Ready
              </>
            )}
          </Button>
        )}

        {activeVersion && isReady && (
          <Button variant="outline" onClick={handleCreateNewVersion} disabled={saving}>
            <Plus className="size-3.5" />
            New Version
          </Button>
        )}
      </div>

      {validationSuccess && (
        <div className="mt-4 rounded-lg border border-status-pass/20 bg-status-pass/5 p-3">
          <p className="text-xs text-status-pass flex items-center gap-1.5">
            <Check className="size-3" />
            All checks passed
          </p>
        </div>
      )}

      {activeVersion && isReady && (
        <div className="mt-10">
          <h2 className="font-[family-name:var(--font-display)] text-lg text-text-1 mb-4 flex items-center gap-2">
            <Rocket className="size-4 text-ds-accent" />
            Create Deployment
          </h2>
          <Card className="bg-card">
            <CardContent className="pt-2">
              <form onSubmit={handleDeploy} className="space-y-4">
                <div className="space-y-2">
                  <Label className="text-xs text-text-2">Deployment Name</Label>
                  <Input
                    value={deployName}
                    onChange={(e) => setDeployName(e.target.value)}
                    placeholder="e.g., my-agent-prod"
                    className="max-w-md"
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-text-2">Runtime Profile ID</Label>
                  <Input
                    value={runtimeProfileId}
                    onChange={(e) => setRuntimeProfileId(e.target.value)}
                    placeholder="UUID"
                    className="max-w-md font-[family-name:var(--font-mono)] text-xs"
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-text-2">Provider Account ID (optional)</Label>
                  <Input
                    value={providerAccountId}
                    onChange={(e) => setProviderAccountId(e.target.value)}
                    placeholder="UUID"
                    className="max-w-md font-[family-name:var(--font-mono)] text-xs"
                  />
                </div>
                <div className="space-y-2">
                  <Label className="text-xs text-text-2">Model Alias ID (optional)</Label>
                  <Input
                    value={modelAliasId}
                    onChange={(e) => setModelAliasId(e.target.value)}
                    placeholder="UUID"
                    className="max-w-md font-[family-name:var(--font-mono)] text-xs"
                  />
                </div>
                <Button type="submit" disabled={deploying || !deployName.trim() || !runtimeProfileId.trim()}>
                  {deploying ? (
                    <>
                      <Loader2 className="size-3.5 animate-spin" />
                      Deploying...
                    </>
                  ) : (
                    <>
                      <Rocket className="size-3.5" />
                      Deploy
                    </>
                  )}
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
