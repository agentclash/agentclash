"use client";

import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";

import { createApiClient } from "@/lib/api/client";
import type {
  CreateRunRankingInsightsRequest,
  ModelAlias,
  ProviderAccount,
  Run,
  RunRankingInsightsResponse,
  RunRankingResponse,
} from "@/lib/api/types";

interface RunRankingInsightsCardProps {
  workspaceId: string;
  run: Run;
  ranking: RunRankingResponse;
}

export function RunRankingInsightsCard({
  workspaceId,
  run,
  ranking,
}: RunRankingInsightsCardProps) {
  const { getAccessToken } = useAccessToken();
  const [providerAccounts, setProviderAccounts] = useState<ProviderAccount[]>([]);
  const [modelAliases, setModelAliases] = useState<ModelAlias[]>([]);
  const [selectedProviderAccountId, setSelectedProviderAccountId] = useState("");
  const [selectedModelAliasId, setSelectedModelAliasId] = useState("");
  const [insights, setInsights] = useState<RunRankingInsightsResponse | null>(null);
  const [loadingOptions, setLoadingOptions] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState("");

  const isEligible =
    run.status === "completed" &&
    run.execution_mode === "comparison" &&
    ranking.state === "ready" &&
    !!ranking.ranking;

  useEffect(() => {
    if (!isEligible) {
      return;
    }

    let cancelled = false;

    async function loadOptions() {
      try {
        setLoadingOptions(true);
        setError("");
        const token = await getAccessToken();
        const api = createApiClient(token);
        const [providerRes, aliasRes] = await Promise.all([
          api.get<{ items: ProviderAccount[] }>(
            `/v1/workspaces/${workspaceId}/provider-accounts`,
          ),
          api.get<{ items: ModelAlias[] }>(
            `/v1/workspaces/${workspaceId}/model-aliases`,
          ),
        ]);
        if (cancelled) {
          return;
        }
        setProviderAccounts(providerRes.items);
        setModelAliases(aliasRes.items);
      } catch (fetchError) {
        if (!cancelled) {
          setError(getErrorMessage(fetchError, "Failed to load insight controls."));
        }
      } finally {
        if (!cancelled) {
          setLoadingOptions(false);
        }
      }
    }

    void loadOptions();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, isEligible, workspaceId]);

  const activeProviderAccounts = providerAccounts.filter(
    (account) => account.status === "active",
  );
  const activeModelAliases = modelAliases.filter((alias) => alias.status === "active");
  const compatibleModelAliases = activeModelAliases.filter((alias) => {
    if (!selectedProviderAccountId) {
      return true;
    }
    return (
      !alias.provider_account_id || alias.provider_account_id === selectedProviderAccountId
    );
  });

  const providerIds = activeProviderAccounts.map((account) => account.id).join(",");
  useEffect(() => {
    if (!activeProviderAccounts.length) {
      setSelectedProviderAccountId("");
      return;
    }
    if (
      selectedProviderAccountId &&
      activeProviderAccounts.some((account) => account.id === selectedProviderAccountId)
    ) {
      return;
    }
    setSelectedProviderAccountId(activeProviderAccounts[0].id);
  }, [activeProviderAccounts, providerIds, selectedProviderAccountId]);

  const aliasIds = compatibleModelAliases.map((alias) => alias.id).join(",");
  useEffect(() => {
    if (!compatibleModelAliases.length) {
      setSelectedModelAliasId("");
      return;
    }
    if (
      selectedModelAliasId &&
      compatibleModelAliases.some((alias) => alias.id === selectedModelAliasId)
    ) {
      return;
    }
    setSelectedModelAliasId(compatibleModelAliases[0].id);
  }, [aliasIds, compatibleModelAliases, selectedModelAliasId]);

  if (!isEligible) {
    return null;
  }

  async function handleGenerateInsights() {
    if (!selectedProviderAccountId || !selectedModelAliasId) {
      return;
    }

    try {
      setGenerating(true);
      setError("");
      const token = await getAccessToken();
      const api = createApiClient(token);
      const response = await api.post<RunRankingInsightsResponse>(
        `/v1/runs/${run.id}/ranking-insights`,
        {
          provider_account_id: selectedProviderAccountId,
          model_alias_id: selectedModelAliasId,
        } satisfies CreateRunRankingInsightsRequest,
      );
      setInsights(response);
    } catch (generationError) {
      setError(
        getErrorMessage(generationError, "Failed to generate ranking insights."),
      );
    } finally {
      setGenerating(false);
    }
  }

  const readyToGenerate =
    !loadingOptions &&
    !generating &&
    !!selectedProviderAccountId &&
    !!selectedModelAliasId;

  return (
    <section className="mb-4 rounded-lg border border-border bg-muted/20 p-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold">Insights</h3>
            <span className="rounded-full border border-border px-2 py-0.5 text-[11px] uppercase tracking-wide text-muted-foreground">
              LLM advisory
            </span>
          </div>
          <p className="text-sm text-muted-foreground">
            Uses current-run ranking evidence only. This is guidance layered on top
            of the deterministic ranking, not a replacement for it.
          </p>
        </div>

        <div className="grid gap-2 md:min-w-[320px]">
          <label className="grid gap-1 text-xs font-medium text-muted-foreground">
            Insight Provider Account
            <select
              aria-label="Insight Provider Account"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
              value={selectedProviderAccountId}
              onChange={(event) => {
                setSelectedProviderAccountId(event.target.value);
                setInsights(null);
                setError("");
              }}
              disabled={loadingOptions || generating || activeProviderAccounts.length === 0}
            >
              <option value="">Select a provider account</option>
              {activeProviderAccounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name} ({account.provider_key})
                </option>
              ))}
            </select>
          </label>

          <label className="grid gap-1 text-xs font-medium text-muted-foreground">
            Insight Model Alias
            <select
              aria-label="Insight Model Alias"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
              value={selectedModelAliasId}
              onChange={(event) => {
                setSelectedModelAliasId(event.target.value);
                setInsights(null);
                setError("");
              }}
              disabled={loadingOptions || generating || compatibleModelAliases.length === 0}
            >
              <option value="">Select a model alias</option>
              {compatibleModelAliases.map((alias) => (
                <option key={alias.id} value={alias.id}>
                  {alias.display_name}
                </option>
              ))}
            </select>
          </label>

          <button
            type="button"
            className="inline-flex h-9 items-center justify-center rounded-md bg-foreground px-3 text-sm font-medium text-background disabled:cursor-not-allowed disabled:opacity-50"
            onClick={() => {
              void handleGenerateInsights();
            }}
            disabled={!readyToGenerate}
          >
            {generating
              ? "Generating insights..."
              : insights
                ? "Regenerate insights"
                : "Generate insights"}
          </button>
        </div>
      </div>

      {loadingOptions ? (
        <div className="mt-4 text-sm text-muted-foreground">
          Loading insight controls...
        </div>
      ) : null}

      {!loadingOptions && activeProviderAccounts.length === 0 ? (
        <div className="mt-4 rounded-md border border-border bg-background p-3 text-sm text-muted-foreground">
          Add an active provider account in this workspace to generate insights.
        </div>
      ) : null}

      {!loadingOptions &&
      activeProviderAccounts.length > 0 &&
      compatibleModelAliases.length === 0 ? (
        <div className="mt-4 rounded-md border border-border bg-background p-3 text-sm text-muted-foreground">
          Add an active model alias that works with the selected provider account.
        </div>
      ) : null}

      {error ? (
        <div className="mt-4 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {insights ? (
        <div className="mt-4 grid gap-4">
          <div className="rounded-md border border-emerald-500/30 bg-emerald-500/5 p-4">
            <div className="text-xs font-medium uppercase tracking-wide text-emerald-700">
              Recommended winner
            </div>
            <div className="mt-1 text-base font-semibold text-foreground">
              {insights.recommended_winner.label}
            </div>
            <p className="mt-2 text-sm text-muted-foreground">
              {insights.why_it_won}
            </p>
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <div className="rounded-md border border-border bg-background p-4">
              <h4 className="text-sm font-medium">Tradeoffs</h4>
              <ul className="mt-2 list-disc space-y-2 pl-5 text-sm text-muted-foreground">
                {insights.tradeoffs.map((tradeoff, index) => (
                  <li key={`${tradeoff}-${index}`}>{tradeoff}</li>
                ))}
              </ul>
            </div>

            <div className="rounded-md border border-border bg-background p-4">
              <h4 className="text-sm font-medium">Recommended next step</h4>
              <p className="mt-2 text-sm text-muted-foreground">
                {insights.recommended_next_step}
              </p>
            </div>
          </div>

          <div className="grid gap-4 lg:grid-cols-3">
            {renderFocusRecommendation(
              "Best for reliability",
              insights.best_for_reliability,
            )}
            {renderFocusRecommendation("Best for cost", insights.best_for_cost)}
            {renderFocusRecommendation(
              "Best for latency",
              insights.best_for_latency,
            )}
          </div>

          <div className="rounded-md border border-border bg-background p-4">
            <h4 className="text-sm font-medium">Lane summaries</h4>
            <div className="mt-3 grid gap-3">
              {insights.model_summaries.map((summary) => (
                <div key={summary.run_agent_id} className="rounded-md border border-border p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium">{summary.label}</span>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                      Strongest: {formatDimensionLabel(summary.strongest_dimension)}
                    </span>
                    <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                      Weakest: {formatDimensionLabel(summary.weakest_dimension)}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">
                    {summary.summary}
                  </p>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-md border border-border bg-background p-4 text-sm text-muted-foreground">
            <div className="font-medium text-foreground">Confidence notes</div>
            <p className="mt-2">{insights.confidence_notes}</p>
            <p className="mt-3 text-xs">
              Generated with {insights.provider_key} / {insights.provider_model_id} at{" "}
              {new Date(insights.generated_at).toLocaleString()}.
            </p>
          </div>
        </div>
      ) : !loadingOptions && !error ? (
        <div className="mt-4 rounded-md border border-border bg-background p-4 text-sm text-muted-foreground">
          No insights yet. Generate an advisory summary to help interpret this
          ranking without losing the underlying metrics.
        </div>
      ) : null}
    </section>
  );
}

function renderFocusRecommendation(
  title: string,
  recommendation?: RunRankingInsightsResponse["best_for_cost"],
) {
  if (!recommendation) {
    return null;
  }

  return (
    <div className="rounded-md border border-border bg-background p-4">
      <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {title}
      </div>
      <div className="mt-1 text-sm font-medium text-foreground">
        {recommendation.label}
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{recommendation.reason}</p>
    </div>
  );
}

function formatDimensionLabel(value: string) {
  if (!value) {
    return "N/A";
  }
  return value.replace(/_/g, " ");
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}
