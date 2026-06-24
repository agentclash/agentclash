"use client";

import { useApiListQuery, usePaginatedApiQuery } from "@/lib/api/swr";
import type {
  AgentDeployment,
  EvalPack,
  ProviderAccount,
  Run,
} from "@/lib/api/types";

/**
 * Web-side mirror of the CLI `agentclash quickstart` readiness model
 * (see cli/cmd/quickstart.go). There is no backend readiness endpoint, so we
 * compose readiness client-side from the same list endpoints the rest of the
 * app already uses.
 *
 * The ordered chain a brand-new workspace must complete before it can run an
 * eval: connect a provider -> deploy an agent -> add a eval pack -> run.
 * (Agent builds are folded into the deploy step: a build is only meaningful as
 * an input to a deployment, so surfacing it separately is friction, not signal.)
 */
export type ReadinessStepKey =
  | "provider"
  | "deployment"
  | "eval_pack"
  | "first_run";

export interface ReadinessStep {
  key: ReadinessStepKey;
  label: string;
  description: string;
  href: string;
  cta: string;
  done: boolean;
}

export interface WorkspaceReadiness {
  steps: ReadinessStep[];
  /** True once a run can be created (provider + deployment + eval pack). */
  ready: boolean;
  /** True once every step — including the first run — is complete. */
  allComplete: boolean;
  /** The first incomplete step, or null when allComplete. */
  nextStep: ReadinessStep | null;
  isLoading: boolean;
  error: boolean;
}

function hasRunnableVersion(pack: EvalPack): boolean {
  return (pack.versions ?? []).some((v) => v.lifecycle_status === "runnable");
}

export function useWorkspaceReadiness(workspaceId: string): WorkspaceReadiness {
  const providers = useApiListQuery<ProviderAccount>(
    `/v1/workspaces/${workspaceId}/provider-accounts`,
  );
  const deployments = useApiListQuery<AgentDeployment>(
    `/v1/workspaces/${workspaceId}/agent-deployments`,
  );
  const packs = useApiListQuery<EvalPack>(
    `/v1/workspaces/${workspaceId}/eval-packs`,
  );
  // Only need to know whether *any* run exists.
  const runs = usePaginatedApiQuery<Run>(`/v1/workspaces/${workspaceId}/runs`, {
    limit: 1,
    offset: 0,
  });

  // No manual memoization: this project's React Compiler lint owns memoization,
  // and the returned object is cheap to recompute (no consumer keys on its
  // identity). SWR dedupes the underlying requests across every caller.
  const hasProvider = (providers.data?.items.length ?? 0) > 0;
  const hasDeployment = (deployments.data?.items ?? []).some(
    (d) => d.status === "active",
  );
  const hasRunnablePack = (packs.data?.items ?? []).some(hasRunnableVersion);
  const hasRun = (runs.data?.total ?? 0) > 0;

  const steps: ReadinessStep[] = [
    {
      key: "provider",
      label: "Connect a provider",
      description:
        "Add an LLM provider account so your agents can call models.",
      href: `/workspaces/${workspaceId}/provider-accounts`,
      cta: "Add provider",
      done: hasProvider,
    },
    {
      key: "deployment",
      label: "Deploy an agent",
      description: "Create an agent deployment to compete in your evals.",
      href: `/workspaces/${workspaceId}/deployments`,
      cta: "Create deployment",
      done: hasDeployment,
    },
    {
      key: "eval_pack",
      label: "Add a eval pack",
      description:
        "Publish a eval pack with a runnable version to benchmark against.",
      href: `/workspaces/${workspaceId}/eval-packs`,
      cta: "Add eval pack",
      done: hasRunnablePack,
    },
    {
      key: "first_run",
      label: "Run your first eval",
      description: "Race your deployments against a eval pack.",
      href: `/workspaces/${workspaceId}/runs`,
      cta: "Create run",
      done: hasRun,
    },
  ];

  const ready = hasProvider && hasDeployment && hasRunnablePack;
  const allComplete = ready && hasRun;
  const nextStep = steps.find((s) => !s.done) ?? null;

  const isLoading =
    (providers.isLoading && !providers.data) ||
    (deployments.isLoading && !deployments.data) ||
    (packs.isLoading && !packs.data) ||
    (runs.isLoading && !runs.data);
  const error = Boolean(
    providers.error || deployments.error || packs.error || runs.error,
  );

  return { steps, ready, allComplete, nextStep, isLoading, error };
}
