"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  AgentDeployment,
  ChallengePack,
  ChallengePackVersion,
  CreateRunRequest,
  CreateRunResponse,
  ListRegressionCasesResponse,
  OfficialPackMode,
  RegressionCase,
  RegressionSuite,
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
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface CreateRunDialogProps {
  workspaceId: string;
}

export function CreateRunDialog({ workspaceId }: CreateRunDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [selectedPackId, setSelectedPackId] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState("");
  const [inputSetId, setInputSetId] = useState("");
  const [selectedDeploymentIds, setSelectedDeploymentIds] = useState<string[]>(
    [],
  );
  const [selectedRegressionSuiteIds, setSelectedRegressionSuiteIds] = useState<
    string[]
  >([]);
  const [selectedRegressionCaseIds, setSelectedRegressionCaseIds] = useState<
    string[]
  >([]);
  const [officialPackMode, setOfficialPackMode] =
    useState<OfficialPackMode>("full");
  const [submitting, setSubmitting] = useState(false);

  const [packs, setPacks] = useState<ChallengePack[]>([]);
  const [runnableVersions, setRunnableVersions] = useState<
    ChallengePackVersion[]
  >([]);
  const [deployments, setDeployments] = useState<AgentDeployment[]>([]);
  const [regressionSuites, setRegressionSuites] = useState<RegressionSuite[]>(
    [],
  );
  const [suiteCases, setSuiteCases] = useState<Record<string, RegressionCase[]>>(
    {},
  );
  const [loading, setLoading] = useState(false);
  const [loadingRegression, setLoadingRegression] = useState(false);
  const [regressionLoadError, setRegressionLoadError] = useState<string | null>(
    null,
  );
  const fetchedSuiteCaseIdsRef = useRef<Set<string>>(new Set());

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const [packsRes, deploymentsRes] = await Promise.all([
        api.get<{ items: ChallengePack[] }>(
          `/v1/workspaces/${workspaceId}/challenge-packs`,
        ),
        api.get<{ items: AgentDeployment[] }>(
          `/v1/workspaces/${workspaceId}/agent-deployments`,
        ),
      ]);
      const suites: RegressionSuite[] = [];
      let offset = 0;
      while (true) {
        const page = await api.paginated<RegressionSuite>(
          `/v1/workspaces/${workspaceId}/regression-suites`,
          { limit: 100, offset },
        );
        suites.push(...page.items);
        offset += page.limit;
        if (suites.length >= page.total || page.items.length === 0) break;
      }
      setPacks(packsRes.items);
      setDeployments(deploymentsRes.items.filter((d) => d.status === "active"));
      setRegressionSuites(suites.filter((suite) => suite.status === "active"));
    } catch {
      toast.error("Failed to load data");
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId]);

  useEffect(() => {
    if (open) loadData();
  }, [open, loadData]);

  function handlePackChange(packId: string) {
    setSelectedPackId(packId);
    setSelectedVersionId("");
    setSelectedRegressionSuiteIds([]);
    setSelectedRegressionCaseIds([]);
    setOfficialPackMode("full");
    setRegressionLoadError(null);
    if (packId) {
      const pack = packs.find((p) => p.id === packId);
      const runnable = (pack?.versions ?? []).filter(
        (v) => v.lifecycle_status === "runnable",
      );
      setRunnableVersions(runnable);
      if (runnable.length === 1) setSelectedVersionId(runnable[0].id);
    } else {
      setRunnableVersions([]);
    }
  }

  function toggleDeployment(id: string) {
    setSelectedDeploymentIds((prev) =>
      prev.includes(id) ? prev.filter((d) => d !== id) : [...prev, id],
    );
  }

  function toggleRegressionSuite(id: string) {
    setSelectedRegressionSuiteIds((prev) =>
      prev.includes(id) ? prev.filter((suiteId) => suiteId !== id) : [...prev, id],
    );
  }

  function toggleRegressionCase(id: string) {
    setSelectedRegressionCaseIds((prev) =>
      prev.includes(id) ? prev.filter((caseId) => caseId !== id) : [...prev, id],
    );
  }

  useEffect(() => {
    if (!open || !selectedPackId) {
      setRegressionLoadError(null);
      return;
    }

    const eligibleSuites = regressionSuites.filter(
      (suite) => suite.source_challenge_pack_id === selectedPackId,
    );
    const missingSuiteIds = eligibleSuites
      .map((suite) => suite.id)
      .filter((suiteId) => !fetchedSuiteCaseIdsRef.current.has(suiteId));

    if (missingSuiteIds.length === 0) return;

    let cancelled = false;
    setLoadingRegression(true);
    setRegressionLoadError(null);
    for (const suiteId of missingSuiteIds) {
      fetchedSuiteCaseIdsRef.current.add(suiteId);
    }

    (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const caseEntries = await Promise.all(
          missingSuiteIds.map(async (suiteId) => {
            const response = await api.get<ListRegressionCasesResponse>(
              `/v1/workspaces/${workspaceId}/regression-suites/${suiteId}/cases`,
            );
            return [
              suiteId,
              response.items.filter((regressionCase) => regressionCase.status === "active"),
            ] as const;
          }),
        );

        if (cancelled) return;

        setSuiteCases((prev) => {
          const next = { ...prev };
          for (const [suiteId, cases] of caseEntries) {
            next[suiteId] = cases;
          }
          return next;
        });
      } catch (err) {
        if (cancelled) return;
        for (const suiteId of missingSuiteIds) {
          fetchedSuiteCaseIdsRef.current.delete(suiteId);
        }
        setRegressionLoadError(
          err instanceof ApiError ? err.message : "Failed to load regression cases",
        );
      } finally {
        if (!cancelled) setLoadingRegression(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [getAccessToken, open, regressionSuites, selectedPackId, workspaceId]);

  useEffect(() => {
    if (selectedRegressionSuiteIds.length === 0 && selectedRegressionCaseIds.length === 0) {
      setOfficialPackMode("full");
    }
  }, [selectedRegressionCaseIds, selectedRegressionSuiteIds]);

  async function handleCreate() {
    if (!selectedVersionId || selectedDeploymentIds.length === 0) return;

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const request: CreateRunRequest = {
        workspace_id: workspaceId,
        challenge_pack_version_id: selectedVersionId,
        challenge_input_set_id: inputSetId.trim() || undefined,
        name: name.trim() || undefined,
        agent_deployment_ids: selectedDeploymentIds,
        regression_suite_ids:
          selectedRegressionSuiteIds.length > 0
            ? selectedRegressionSuiteIds
            : undefined,
        regression_case_ids:
          selectedRegressionCaseIds.length > 0
            ? selectedRegressionCaseIds
            : undefined,
        official_pack_mode:
          selectedRegressionSuiteIds.length > 0 ||
          selectedRegressionCaseIds.length > 0
            ? officialPackMode
            : undefined,
      };
      const result = await api.post<CreateRunResponse>("/v1/runs", request);
      toast.success("Run created");
      setOpen(false);
      resetForm();
      router.push(`/workspaces/${workspaceId}/runs/${result.id}`);
      router.refresh();
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create run",
      );
    } finally {
      setSubmitting(false);
    }
  }

  function resetForm() {
    setName("");
    setSelectedPackId("");
    setSelectedVersionId("");
    setInputSetId("");
    setSelectedDeploymentIds([]);
    setSelectedRegressionSuiteIds([]);
    setSelectedRegressionCaseIds([]);
    setOfficialPackMode("full");
    setRegressionLoadError(null);
    setRunnableVersions([]);
  }

  const executionMode =
    selectedDeploymentIds.length > 1 ? "comparison" : "single_agent";
  const canSubmit = selectedVersionId && selectedDeploymentIds.length > 0;
  const eligibleRegressionSuites = regressionSuites.filter(
    (suite) => suite.source_challenge_pack_id === selectedPackId,
  );
  const regressionSelectionCount =
    selectedRegressionSuiteIds.length + selectedRegressionCaseIds.length;

  const selectClass =
    "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" />}>
        <Plus data-icon="inline-start" className="size-4" />
        New Run
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New Run</DialogTitle>
          <DialogDescription>
            Run agent deployments against a challenge pack.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
          {/* Name */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Name{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Auto-generated if empty"
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
            />
          </div>

          {/* Challenge Pack */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Challenge Pack
            </label>
            <select
              aria-label="Challenge Pack"
              value={selectedPackId}
              onChange={(e) => handlePackChange(e.target.value)}
              disabled={loading}
              className={selectClass}
            >
              <option value="">
                {loading ? "Loading..." : "Select a challenge pack"}
              </option>
              {packs.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>

          {/* Pack Version */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Version{" "}
              <span className="text-muted-foreground font-normal">
                (runnable only)
              </span>
            </label>
            <select
              aria-label="Challenge Pack Version"
              value={selectedVersionId}
              onChange={(e) => setSelectedVersionId(e.target.value)}
              disabled={!selectedPackId}
              className={selectClass}
            >
              <option value="">
                {runnableVersions.length === 0 && selectedPackId
                  ? "No runnable versions"
                  : "Select a version"}
              </option>
              {runnableVersions.map((v) => (
                <option key={v.id} value={v.id}>
                  v{v.version_number}
                </option>
              ))}
            </select>
          </div>

          {/* Input Set ID (optional) */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Input Set ID{" "}
              <span className="text-muted-foreground font-normal">
                (optional)
              </span>
            </label>
            <input
              type="text"
              value={inputSetId}
              onChange={(e) => setInputSetId(e.target.value)}
              placeholder="UUID — leave empty for all input sets"
              className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm font-[family-name:var(--font-mono)] placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50"
            />
          </div>

          <div className="space-y-3 rounded-lg border border-border bg-muted/20 p-3">
            <div>
              <p className="text-sm font-medium">Regression Coverage</p>
              <p className="text-xs text-muted-foreground">
                Optionally add regression suites or specific cases to this run.
              </p>
            </div>

            {!selectedPackId ? (
              <p className="text-sm text-muted-foreground">
                Select a challenge pack to load matching regression suites.
              </p>
            ) : loadingRegression ? (
              <p className="text-sm text-muted-foreground">
                Loading regression suites and cases...
              </p>
            ) : regressionLoadError ? (
              <p className="text-sm text-destructive">{regressionLoadError}</p>
            ) : eligibleRegressionSuites.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No active regression suites are linked to this challenge pack yet.
              </p>
            ) : (
              <div className="space-y-3">
                <div className="space-y-2 rounded-lg border border-input p-2">
                  {eligibleRegressionSuites.map((suite) => {
                    const cases = suiteCases[suite.id] ?? [];
                    return (
                      <div key={suite.id} className="space-y-2 rounded-md border border-border/60 p-2">
                        <label className="flex items-start gap-2 text-sm cursor-pointer">
                          <input
                            type="checkbox"
                            checked={selectedRegressionSuiteIds.includes(suite.id)}
                            onChange={() => toggleRegressionSuite(suite.id)}
                            className="mt-0.5 rounded border-input"
                          />
                          <span className="flex-1">
                            <span className="font-medium text-foreground">
                              {suite.name}
                            </span>
                            <span className="ml-2 text-xs text-muted-foreground">
                              {suite.case_count} case{suite.case_count === 1 ? "" : "s"}
                            </span>
                            {suite.description && (
                              <span className="mt-0.5 block text-xs text-muted-foreground">
                                {suite.description}
                              </span>
                            )}
                          </span>
                        </label>

                        {cases.length > 0 && (
                          <div className="space-y-1 border-l border-border pl-4">
                            {cases.map((regressionCase) => (
                              <label
                                key={regressionCase.id}
                                className="flex items-start gap-2 text-sm cursor-pointer"
                              >
                                <input
                                  type="checkbox"
                                  checked={selectedRegressionCaseIds.includes(regressionCase.id)}
                                  onChange={() => toggleRegressionCase(regressionCase.id)}
                                  className="mt-0.5 rounded border-input"
                                />
                                <span className="flex-1">
                                  <span className="text-foreground">
                                    {regressionCase.title}
                                  </span>
                                  <span className="ml-2 text-xs text-muted-foreground">
                                    {regressionCase.severity}
                                  </span>
                                  {regressionCase.failure_summary && (
                                    <span className="mt-0.5 block text-xs text-muted-foreground">
                                      {regressionCase.failure_summary}
                                    </span>
                                  )}
                                </span>
                              </label>
                            ))}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>

                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    Official Pack Mode
                  </label>
                  <select
                    aria-label="Official Pack Mode"
                    value={officialPackMode}
                    onChange={(e) =>
                      setOfficialPackMode(e.target.value as OfficialPackMode)
                    }
                    disabled={regressionSelectionCount === 0}
                    className={selectClass}
                  >
                    <option value="full">
                      Full - run official pack plus selected regressions
                    </option>
                    <option value="suite_only">
                      Suite only - run only the selected regressions
                    </option>
                  </select>
                </div>
              </div>
            )}
          </div>

          {/* Agent Deployments (multi-select) */}
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Agent Deployments
            </label>
            {loading ? (
              <p className="text-sm text-muted-foreground">Loading...</p>
            ) : deployments.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                No active deployments — create one first.
              </p>
            ) : (
              <div className="space-y-1.5 rounded-lg border border-input p-2 max-h-40 overflow-y-auto">
                {deployments.map((d) => (
                  <label
                    key={d.id}
                    className="flex items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-muted/50 cursor-pointer"
                  >
                    <input
                      type="checkbox"
                      checked={selectedDeploymentIds.includes(d.id)}
                      onChange={() => toggleDeployment(d.id)}
                      className="rounded border-input"
                    />
                    <span className="truncate">{d.name}</span>
                  </label>
                ))}
              </div>
            )}
          </div>

          {/* Preview */}
          {canSubmit && (
            <div className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              This will run{" "}
              <span className="text-foreground font-medium">
                {selectedDeploymentIds.length} agent
                {selectedDeploymentIds.length !== 1 ? "s" : ""}
              </span>{" "}
              against the selected challenge pack in{" "}
              <span className="text-foreground font-medium">
                {executionMode === "comparison"
                  ? "comparison"
                  : "single-agent"}{" "}
                mode
              </span>
              {regressionSelectionCount > 0 && (
                <>
                  {" "}with{" "}
                  <span className="text-foreground font-medium">
                    {regressionSelectionCount} regression selection
                    {regressionSelectionCount === 1 ? "" : "s"}
                  </span>{" "}
                  in{" "}
                  <span className="text-foreground font-medium">
                    {officialPackMode === "suite_only" ? "suite-only" : "full"}
                  </span>{" "}
                  mode
                </>
              )}
              .
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => setOpen(false)}
            disabled={submitting}
          >
            Cancel
          </Button>
          <Button disabled={!canSubmit || submitting} onClick={handleCreate}>
            {submitting ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              "Create Run"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
