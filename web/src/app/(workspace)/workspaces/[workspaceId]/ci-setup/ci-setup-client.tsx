"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import {
  CheckCircle2,
  Clipboard,
  Download,
  FileCode2,
  GitBranch,
  Github,
  Loader2,
  Play,
  ShieldCheck,
  TriangleAlert,
} from "lucide-react";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import {
  useApiListQuery,
  usePaginatedApiQuery,
} from "@/lib/api/swr";
import {
  ciSetupReadiness,
  defaultCISetupConfig,
  generateAgentClashCIManifest,
  generateAgentClashGitHubWorkflow,
  type CIBaselineRefresh,
  type CIBaselineStrategy,
  type CIGateFailOn,
  type CIRegressionPromotion,
  type CISetupConfig,
} from "@/lib/ci-setup";
import type {
  AgentBuild,
  AgentDeployment,
  CIProfile,
  ChallengeInputSetSummary,
  ChallengePack,
  CISetupFileConflict,
  CreateCISetupPullRequestRequest,
  CreateCISetupPullRequestResponse,
  GitHubRepository,
  ModelAlias,
  ProviderAccount,
  RegressionSuite,
  Run,
  RunAgent,
  RuntimeProfile,
  SaveCIProfileRequest,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { cn } from "@/lib/utils";

interface CISetupClientProps {
  workspaceId: string;
}

const textAreaClass =
  "block min-h-24 w-full rounded-lg border border-input bg-transparent px-3 py-2 font-mono text-xs leading-5 placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50";
const selectClass =
  "block h-9 w-full rounded-lg border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50";

export function CISetupClient({ workspaceId }: CISetupClientProps) {
  const { getAccessToken } = useAccessToken();
  const [workflowPath, setWorkflowPath] = useState(
    defaultCISetupConfig.workflowPath,
  );
  const [manifestPath, setManifestPath] = useState(
    defaultCISetupConfig.manifestPath,
  );
  const [agentSpecPath, setAgentSpecPath] = useState(
    defaultCISetupConfig.agentSpecPath,
  );
  const [triggerPathsText, setTriggerPathsText] = useState(
    defaultCISetupConfig.triggerPaths.join("\n"),
  );
  const [triggerLabelsText, setTriggerLabelsText] = useState(
    defaultCISetupConfig.triggerLabels.join("\n"),
  );
  const [repositoryFullName, setRepositoryFullName] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");
  const [agentBuildId, setAgentBuildId] = useState("");
  const [runtimeProfileId, setRuntimeProfileId] = useState("");
  const [providerAccountId, setProviderAccountId] = useState("");
  const [modelAliasId, setModelAliasId] = useState("");
  const [deploymentName, setDeploymentName] = useState("pr-candidate");
  const [selectedPackId, setSelectedPackId] = useState("");
  const [selectedVersionId, setSelectedVersionId] = useState("");
  const [inputSetId, setInputSetId] = useState("");
  const [selectedRegressionSuiteIds, setSelectedRegressionSuiteIds] = useState<
    string[]
  >([]);
  const [baselineStrategy, setBaselineStrategy] =
    useState<CIBaselineStrategy>("locked_run");
  const [baselineRunId, setBaselineRunId] = useState("");
  const [baselineRunIdManualOverride, setBaselineRunIdManualOverride] =
    useState(false);
  const [baselineRunAgentId, setBaselineRunAgentId] = useState("");
  const [baselineDeploymentId, setBaselineDeploymentId] = useState("");
  const [baselineRefresh, setBaselineRefresh] =
    useState<CIBaselineRefresh>("manual");
  const [baselineMaxAgeDays, setBaselineMaxAgeDays] = useState(30);
  const [gateFailOn, setGateFailOn] = useState<CIGateFailOn>("regression");
  const [gatePolicyFile, setGatePolicyFile] = useState("");
  const [regressionPromotion, setRegressionPromotion] =
    useState<CIRegressionPromotion>("proposed");
  const [inputSets, setInputSets] = useState<ChallengeInputSetSummary[]>([]);
  const [runAgents, setRunAgents] = useState<RunAgent[]>([]);
  const [loadingInputSets, setLoadingInputSets] = useState(false);
  const [loadingRunAgents, setLoadingRunAgents] = useState(false);
  const [selectedRepositoryKey, setSelectedRepositoryKey] = useState("");
  const [selectedProfileId, setSelectedProfileId] = useState("");
  const [profileName, setProfileName] = useState("Default CI profile");
  const [savingProfile, setSavingProfile] = useState(false);
  const [creatingSetupPR, setCreatingSetupPR] = useState(false);
  const [setupPRResult, setSetupPRResult] =
    useState<CreateCISetupPullRequestResponse | null>(null);
  const [setupPRConflicts, setSetupPRConflicts] = useState<CISetupFileConflict[]>(
    [],
  );
  const [overwriteExisting, setOverwriteExisting] = useState(false);

  const builds = useApiListQuery<AgentBuild>(
    `/v1/workspaces/${workspaceId}/agent-builds`,
  );
  const deployments = useApiListQuery<AgentDeployment>(
    `/v1/workspaces/${workspaceId}/agent-deployments`,
  );
  const packs = useApiListQuery<ChallengePack>(
    `/v1/workspaces/${workspaceId}/challenge-packs`,
  );
  const runtimeProfiles = useApiListQuery<RuntimeProfile>(
    `/v1/workspaces/${workspaceId}/runtime-profiles`,
  );
  const providerAccounts = useApiListQuery<ProviderAccount>(
    `/v1/workspaces/${workspaceId}/provider-accounts`,
  );
  const modelAliases = useApiListQuery<ModelAlias>(
    `/v1/workspaces/${workspaceId}/model-aliases`,
  );
  const regressionSuites = useApiListQuery<RegressionSuite>(
    `/v1/workspaces/${workspaceId}/regression-suites`,
    { limit: 100, offset: 0 },
  );
  const repositories = useApiListQuery<GitHubRepository>(
    `/v1/workspaces/${workspaceId}/github/repositories`,
  );
  const ciProfiles = useApiListQuery<CIProfile>(
    `/v1/workspaces/${workspaceId}/ci-profiles`,
  );
  const runs = usePaginatedApiQuery<Run>(`/v1/workspaces/${workspaceId}/runs`, {
    limit: 100,
    offset: 0,
    status: "completed",
  });

  const activeBuilds = useMemo(
    () =>
      (builds.data?.items ?? []).filter(
        (item) => item.lifecycle_status === "active",
      ),
    [builds.data?.items],
  );
  const activeDeployments = useMemo(
    () => (deployments.data?.items ?? []).filter((item) => item.status === "active"),
    [deployments.data?.items],
  );
  const runtimeProfileOptions = useMemo(
    () => runtimeProfiles.data?.items ?? [],
    [runtimeProfiles.data?.items],
  );
  const activeProviderAccounts = useMemo(
    () => (providerAccounts.data?.items ?? []).filter((item) => item.status === "active"),
    [providerAccounts.data?.items],
  );
  const activeModelAliases = useMemo(
    () => (modelAliases.data?.items ?? []).filter((item) => item.status === "active"),
    [modelAliases.data?.items],
  );
  const challengePacks = useMemo(
    () => packs.data?.items ?? [],
    [packs.data?.items],
  );
  const selectedPack = challengePacks.find((pack) => pack.id === selectedPackId);
  const runnableVersions = useMemo(
    () =>
      (selectedPack?.versions ?? []).filter(
        (version) => version.lifecycle_status === "runnable",
      ),
    [selectedPack],
  );
  const activeRegressionSuites = useMemo(
    () =>
      (regressionSuites.data?.items ?? []).filter(
        (suite) =>
          suite.status === "active" &&
          (!selectedPackId || suite.source_challenge_pack_id === selectedPackId),
      ),
    [regressionSuites.data?.items, selectedPackId],
  );
  const completedRuns = useMemo(
    () => (runs.data?.items ?? []).filter((run) => run.status === "completed"),
    [runs.data?.items],
  );
  const githubRepositories = useMemo(
    () => repositories.data?.items ?? [],
    [repositories.data?.items],
  );
  const selectedGitHubRepository = useMemo(
    () =>
      githubRepositories.find((repo) => gitHubRepositoryKey(repo) === selectedRepositoryKey) ??
      null,
    [githubRepositories, selectedRepositoryKey],
  );

  useEffect(() => {
    const repository = githubRepositories[0];
    if (repository && !repositoryFullName) {
      setSelectedRepositoryKey(gitHubRepositoryKey(repository));
      setRepositoryFullName(repository.full_name);
      setDefaultBranch(repository.default_branch || "main");
    }
  }, [githubRepositories, repositoryFullName]);

  useEffect(() => {
    if (!selectedRepositoryKey || !repositories.data) return;
    const repository = githubRepositories.find(
      (repo) => gitHubRepositoryKey(repo) === selectedRepositoryKey,
    );
    if (!repository) {
      setSelectedRepositoryKey("");
    }
  }, [githubRepositories, repositories.data, selectedRepositoryKey]);

  useEffect(() => {
    if (!builds.data) return;
    if (!activeBuilds.some((build) => build.id === agentBuildId)) {
      setAgentBuildId(activeBuilds[0]?.id ?? "");
    }
  }, [activeBuilds, agentBuildId, builds.data]);

  useEffect(() => {
    if (!runtimeProfiles.data) return;
    if (!runtimeProfileOptions.some((profile) => profile.id === runtimeProfileId)) {
      setRuntimeProfileId(runtimeProfileOptions[0]?.id ?? "");
    }
  }, [runtimeProfileId, runtimeProfileOptions, runtimeProfiles.data]);

  useEffect(() => {
    if (!deployments.data) return;
    if (
      !activeDeployments.some(
        (deployment) => deployment.id === baselineDeploymentId,
      )
    ) {
      setBaselineDeploymentId(activeDeployments[0]?.id ?? "");
    }
  }, [activeDeployments, baselineDeploymentId, deployments.data]);

  useEffect(() => {
    if (!runs.data || baselineRunIdManualOverride) return;
    if (!completedRuns.some((run) => run.id === baselineRunId)) {
      setBaselineRunId(completedRuns[0]?.id ?? "");
    }
  }, [baselineRunId, baselineRunIdManualOverride, completedRuns, runs.data]);

  useEffect(() => {
    if (!packs.data) return;
    if (!challengePacks.some((pack) => pack.id === selectedPackId)) {
      setSelectedPackId(challengePacks[0]?.id ?? "");
    }
  }, [challengePacks, packs.data, selectedPackId]);

  useEffect(() => {
    if (!selectedPack) return;
    if (!runnableVersions.some((version) => version.id === selectedVersionId)) {
      setSelectedVersionId(runnableVersions[0]?.id ?? "");
      setInputSetId("");
      setSelectedRegressionSuiteIds([]);
    }
  }, [runnableVersions, selectedPack, selectedVersionId]);

  useEffect(() => {
    if (!providerAccounts.data) return;
    if (
      providerAccountId &&
      !activeProviderAccounts.some((account) => account.id === providerAccountId)
    ) {
      setProviderAccountId("");
    }
  }, [activeProviderAccounts, providerAccountId, providerAccounts.data]);

  useEffect(() => {
    if (!modelAliases.data) return;
    if (
      modelAliasId &&
      !activeModelAliases.some((alias) => alias.id === modelAliasId)
    ) {
      setModelAliasId("");
    }
  }, [activeModelAliases, modelAliasId, modelAliases.data]);

  useEffect(() => {
    if (!regressionSuites.data) return;
    setSelectedRegressionSuiteIds((current) => {
      const next = current.filter((id) =>
        activeRegressionSuites.some((suite) => suite.id === id),
      );
      return next.length === current.length ? current : next;
    });
  }, [activeRegressionSuites, regressionSuites.data]);

  useEffect(() => {
    if (!selectedVersionId) {
      setInputSets([]);
      setInputSetId("");
      return;
    }
    let cancelled = false;
    setLoadingInputSets(true);
    setInputSets([]);
    setInputSetId("");
    void (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token ?? undefined);
        const response = await api.get<{ items: ChallengeInputSetSummary[] }>(
          `/v1/workspaces/${workspaceId}/challenge-pack-versions/${selectedVersionId}/input-sets`,
        );
        if (cancelled) return;
        setInputSets(response.items);
        if (response.items.length === 1) setInputSetId(response.items[0].id);
      } catch (err) {
        if (!cancelled) {
          toast.error(
            err instanceof ApiError ? err.message : "Failed to load input sets",
          );
        }
      } finally {
        if (!cancelled) setLoadingInputSets(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, selectedVersionId, workspaceId]);

  useEffect(() => {
    if (!baselineRunId) {
      setRunAgents([]);
      setBaselineRunAgentId("");
      return;
    }
    let cancelled = false;
    setLoadingRunAgents(true);
    setRunAgents([]);
    setBaselineRunAgentId("");
    void (async () => {
      try {
        const token = await getAccessToken();
        const api = createApiClient(token ?? undefined);
        const response = await api.get<{ items: RunAgent[] }>(
          `/v1/runs/${baselineRunId}/agents`,
        );
        if (cancelled) return;
        setRunAgents(response.items);
        if (response.items.length === 1) setBaselineRunAgentId(response.items[0].id);
      } catch (err) {
        if (!cancelled) {
          toast.error(
            err instanceof ApiError
              ? err.message
              : "Failed to load baseline run agents",
          );
        }
      } finally {
        if (!cancelled) setLoadingRunAgents(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [baselineRunId, getAccessToken]);

  const config: CISetupConfig = {
    ...defaultCISetupConfig,
    repositoryFullName,
    defaultBranch,
    workflowPath,
    manifestPath,
    agentSpecPath,
    triggerPaths: splitLines(triggerPathsText),
    triggerLabels: splitLines(triggerLabelsText),
    agentBuildId,
    runtimeProfileId,
    deploymentName,
    providerAccountId: providerAccountId || undefined,
    modelAliasId: modelAliasId || undefined,
    challengePackVersionId: selectedVersionId,
    inputSetId: inputSetId || undefined,
    regressionSuiteIds: selectedRegressionSuiteIds,
    regressionCaseIds: [],
    baselineStrategy,
    baselineRunId: baselineRunId || undefined,
    baselineRunAgentId: baselineRunAgentId || undefined,
    baselineDeploymentId: baselineDeploymentId || undefined,
    baselineRefresh,
    baselineMaxAgeDays,
    gateFailOn,
    gatePolicyFile: gatePolicyFile || undefined,
    regressionPromotion,
  };

  const readiness = ciSetupReadiness(config);
  const manifest = generateAgentClashCIManifest(config);
  const workflow = generateAgentClashGitHubWorkflow(config);
  const loadingAny =
    builds.isLoading ||
    deployments.isLoading ||
    packs.isLoading ||
    runtimeProfiles.isLoading ||
    providerAccounts.isLoading ||
    modelAliases.isLoading ||
    regressionSuites.isLoading ||
    repositories.isLoading ||
    ciProfiles.isLoading ||
    runs.isLoading;
  const canCreateSetupPR =
    readiness.ready &&
    selectedGitHubRepository !== null &&
    !loadingAny &&
    !creatingSetupPR;
  const loadError =
    builds.error ||
    deployments.error ||
    packs.error ||
    runtimeProfiles.error ||
    providerAccounts.error ||
    modelAliases.error ||
    regressionSuites.error ||
    repositories.error ||
    ciProfiles.error ||
    runs.error;

  const savedProfiles = ciProfiles.data?.items ?? [];
  const selectedProfile =
    savedProfiles.find((profile) => profile.id === selectedProfileId) ?? null;

  async function createSetupPullRequest() {
    if (!selectedGitHubRepository) {
      toast.error("Select an installed GitHub repository first");
      return;
    }
    setCreatingSetupPR(true);
    setSetupPRResult(null);
    setSetupPRConflicts([]);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const payload: CreateCISetupPullRequestRequest = {
        github_repository_id: selectedGitHubRepository.github_repository_id,
        github_installation_id: selectedGitHubRepository.github_installation_id,
        base_branch: config.defaultBranch,
        title: "Set up AgentClash CI",
        body: [
          "Adds AgentClash CI configuration generated from the workspace setup UI.",
          "",
          `Manifest: \`${config.manifestPath}\``,
          `Workflow: \`${config.workflowPath}\``,
        ].join("\n"),
        draft: true,
        overwrite_existing: overwriteExisting,
        files: [
          { path: config.manifestPath, content: manifest },
          { path: config.workflowPath, content: workflow },
        ],
      };
      const result = await api.post<CreateCISetupPullRequestResponse>(
        `/v1/workspaces/${workspaceId}/github/ci-setup-pull-request`,
        payload,
      );
      if (!result.pull_request) {
        setSetupPRConflicts(result.conflicts ?? []);
        if ((result.conflicts ?? []).length > 0) {
          toast.warning("Generated files already exist. Confirm overwrite to open a setup PR.");
        }
        return;
      }
      setSetupPRResult(result);
      setOverwriteExisting(false);
      toast.success(`Created setup PR #${result.pull_request.number}`);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to create setup PR",
      );
    } finally {
      setCreatingSetupPR(false);
    }
  }

  async function saveCIProfile() {
    const name = profileName.trim();
    if (!name) {
      toast.error("Name the CI profile before saving");
      return;
    }
    setSavingProfile(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token ?? undefined);
      const payload: SaveCIProfileRequest = {
        name,
        repository_full_name: config.repositoryFullName,
        github_repository_id: selectedGitHubRepository?.github_repository_id,
        github_installation_id: selectedGitHubRepository?.github_installation_id,
        default_branch: config.defaultBranch,
        manifest_path: config.manifestPath,
        workflow_path: config.workflowPath,
        config: buildProfileConfig({
          selectedRepositoryKey,
          selectedPackId,
          selectedRegressionSuiteIds,
          triggerPathsText,
          triggerLabelsText,
          workflowPath,
          manifestPath,
          agentSpecPath,
          repositoryFullName,
          defaultBranch,
          agentBuildId,
          runtimeProfileId,
          providerAccountId,
          modelAliasId,
          deploymentName,
          selectedVersionId,
          inputSetId,
          baselineStrategy,
          baselineRunId,
          baselineRunAgentId,
          baselineDeploymentId,
          baselineRefresh,
          baselineMaxAgeDays,
          gateFailOn,
          gatePolicyFile,
          regressionPromotion,
        }),
      };
      let saved: CIProfile;
      if (selectedProfileId) {
        saved = await api.patch<CIProfile>(
          `/v1/workspaces/${workspaceId}/ci-profiles/${selectedProfileId}`,
          payload,
        );
      } else {
        saved = await api.post<CIProfile>(
          `/v1/workspaces/${workspaceId}/ci-profiles`,
          payload,
        );
      }
      setSelectedProfileId(saved.id);
      setProfileName(saved.name);
      await ciProfiles.mutate?.();
      toast.success("CI profile saved");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save CI profile");
    } finally {
      setSavingProfile(false);
    }
  }

  function applyProfile(profile: CIProfile | null) {
    if (!profile) return;
    const stored = normalizeProfileConfig(profile.config);
    setSelectedProfileId(profile.id);
    setProfileName(profile.name);
    setSelectedRepositoryKey(stringValue(stored.selectedRepositoryKey, ""));
    setWorkflowPath(stringValue(stored.workflowPath, profile.workflow_path));
    setManifestPath(stringValue(stored.manifestPath, profile.manifest_path));
    setAgentSpecPath(
      stringValue(stored.agentSpecPath, defaultCISetupConfig.agentSpecPath),
    );
    setTriggerPathsText(
      stringArrayValue(stored.triggerPaths, defaultCISetupConfig.triggerPaths).join(
        "\n",
      ),
    );
    setTriggerLabelsText(
      stringArrayValue(
        stored.triggerLabels,
        defaultCISetupConfig.triggerLabels,
      ).join("\n"),
    );
    setRepositoryFullName(
      stringValue(stored.repositoryFullName, profile.repository_full_name),
    );
    setDefaultBranch(stringValue(stored.defaultBranch, profile.default_branch));
    setAgentBuildId(stringValue(stored.agentBuildId, ""));
    setRuntimeProfileId(stringValue(stored.runtimeProfileId, ""));
    setProviderAccountId(stringValue(stored.providerAccountId, ""));
    setModelAliasId(stringValue(stored.modelAliasId, ""));
    setDeploymentName(stringValue(stored.deploymentName, "pr-candidate"));
    setSelectedPackId(stringValue(stored.selectedPackId, ""));
    setSelectedVersionId(stringValue(stored.selectedVersionId, ""));
    setInputSetId(stringValue(stored.inputSetId, ""));
    setSelectedRegressionSuiteIds(
      stringArrayValue(stored.selectedRegressionSuiteIds, []),
    );
    setBaselineStrategy(
      stringValue(stored.baselineStrategy, "locked_run") as CIBaselineStrategy,
    );
    setBaselineRunId(stringValue(stored.baselineRunId, ""));
    setBaselineRunIdManualOverride(Boolean(stored.baselineRunId));
    setBaselineRunAgentId(stringValue(stored.baselineRunAgentId, ""));
    setBaselineDeploymentId(stringValue(stored.baselineDeploymentId, ""));
    setBaselineRefresh(
      stringValue(stored.baselineRefresh, "manual") as CIBaselineRefresh,
    );
    setBaselineMaxAgeDays(numberValue(stored.baselineMaxAgeDays, 30));
    setGateFailOn(stringValue(stored.gateFailOn, "regression") as CIGateFailOn);
    setGatePolicyFile(stringValue(stored.gatePolicyFile, ""));
    setRegressionPromotion(
      stringValue(stored.regressionPromotion, "proposed") as CIRegressionPromotion,
    );
    toast.success(`Loaded ${profile.name}`);
  }

  return (
    <div className="mx-auto flex max-w-7xl flex-col gap-6">
      <header className="flex flex-col gap-4 border-b border-border pb-5 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
            <ShieldCheck className="size-4" />
            CI setup
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">
            AgentClash GitHub Actions gate
          </h1>
          <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">
            Configure a repo-tracked AgentClash gate from workspace resources and
            generate the two files your repository needs.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <ReadinessBadge ready={readiness.ready} loading={loadingAny} />
          <Button
            variant="outline"
            size="sm"
            onClick={() => downloadText(config.manifestPath, manifest)}
          >
            <Download data-icon="inline-start" className="size-4" />
            Manifest
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => downloadText(config.workflowPath, workflow)}
          >
            <Download data-icon="inline-start" className="size-4" />
            Workflow
          </Button>
          <Button
            size="sm"
            disabled={!canCreateSetupPR}
            onClick={createSetupPullRequest}
          >
            {creatingSetupPR ? (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            ) : (
              <Github data-icon="inline-start" className="size-4" />
            )}
            Open setup PR
          </Button>
        </div>
      </header>

      {loadError ? (
        <StatusPanel
          tone="danger"
          title="Failed to load one or more workspace resources."
          body="Refresh the page or confirm this workspace still has access to builds, runs, deployments, and GitHub repositories."
        />
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.02fr)_minmax(420px,0.98fr)]">
        <div className="space-y-6">
          <Section
            icon={<ShieldCheck className="size-4" />}
            title="Saved Profile"
            meta={savedProfiles.length ? `${savedProfiles.length} saved` : "New"}
          >
            <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto]">
              <Field label="Profile">
                <select
                  value={selectedProfileId}
                  onChange={(event) => {
                    const value = event.target.value;
                    setSelectedProfileId(value);
                    const profile = savedProfiles.find((item) => item.id === value);
                    if (profile) {
                      applyProfile(profile);
                    } else {
                      setProfileName("Default CI profile");
                    }
                  }}
                  className={selectClass}
                >
                  <option value="">Create a new profile</option>
                  {savedProfiles.map((profile) => (
                    <option key={profile.id} value={profile.id}>
                      {profile.name}
                    </option>
                  ))}
                </select>
              </Field>
              <div className="flex items-end">
                <Button
                  type="button"
                  variant="outline"
                  disabled={!selectedProfile}
                  onClick={() => applyProfile(selectedProfile)}
                >
                  <Download data-icon="inline-start" className="size-4" />
                  Load profile
                </Button>
              </div>
              <Field label="Profile name">
                <Input
                  value={profileName}
                  onChange={(event) => setProfileName(event.target.value)}
                />
              </Field>
              <div className="flex items-end">
                <Button type="button" disabled={savingProfile} onClick={saveCIProfile}>
                  {savingProfile ? (
                    <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
                  ) : (
                    <CheckCircle2 data-icon="inline-start" className="size-4" />
                  )}
                  Save profile
                </Button>
              </div>
            </div>
          </Section>

          <Section
            icon={<Github className="size-4" />}
            title="Repository"
            meta={repositories.data?.items?.length ? "GitHub connected" : "Manual entry"}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Repository">
                <select
                  value={selectedRepositoryKey}
                  onChange={(event) => {
                    const value = event.target.value;
                    setSelectedRepositoryKey(value);
                    const repo = githubRepositories.find(
                      (item) => gitHubRepositoryKey(item) === value,
                    );
                    if (repo) {
                      setRepositoryFullName(repo.full_name);
                      setDefaultBranch(repo.default_branch || "main");
                    } else {
                      setRepositoryFullName("");
                    }
                  }}
                  className={selectClass}
                >
                  <option value="">Enter manually below</option>
                  {githubRepositories.map((repo) => (
                    <option key={repo.id} value={gitHubRepositoryKey(repo)}>
                      {repo.full_name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Default branch">
                <Input
                  value={defaultBranch}
                  onChange={(event) => setDefaultBranch(event.target.value)}
                  placeholder="main"
                />
              </Field>
              <Field label="Repository full name">
                <Input
                  value={repositoryFullName}
                  onChange={(event) => {
                    setSelectedRepositoryKey("");
                    setRepositoryFullName(event.target.value);
                  }}
                  placeholder="owner/repo"
                />
              </Field>
              <Field label="Workflow file">
                <Input
                  value={workflowPath}
                  onChange={(event) => setWorkflowPath(event.target.value)}
                />
              </Field>
            </div>
          </Section>

          <Section
            icon={<GitBranch className="size-4" />}
            title="Trigger Policy"
            meta={`${splitLines(triggerPathsText).length} paths`}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Watched paths">
                <textarea
                  value={triggerPathsText}
                  onChange={(event) => setTriggerPathsText(event.target.value)}
                  className={textAreaClass}
                />
              </Field>
              <Field label="Force-run labels">
                <textarea
                  value={triggerLabelsText}
                  onChange={(event) => setTriggerLabelsText(event.target.value)}
                  className={textAreaClass}
                />
              </Field>
              <Field label="Manifest file">
                <Input
                  value={manifestPath}
                  onChange={(event) => setManifestPath(event.target.value)}
                />
              </Field>
              <Field label="Candidate spec file">
                <Input
                  value={agentSpecPath}
                  onChange={(event) => {
                    const next = event.target.value;
                    setAgentSpecPath(next);
                    if (!splitLines(triggerPathsText).includes(next)) {
                      setTriggerPathsText((current) =>
                        [next, ...splitLines(current)].filter(Boolean).join("\n"),
                      );
                    }
                  }}
                />
              </Field>
            </div>
          </Section>

          <Section
            icon={<FileCode2 className="size-4" />}
            title="Candidate"
            meta={activeBuilds.length ? `${activeBuilds.length} active builds` : "No builds"}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Agent build">
                <select
                  value={agentBuildId}
                  onChange={(event) => setAgentBuildId(event.target.value)}
                  className={selectClass}
                >
                  <option value="">Select build</option>
                  {activeBuilds.map((build) => (
                    <option key={build.id} value={build.id}>
                      {build.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Candidate deployment name">
                <Input
                  value={deploymentName}
                  onChange={(event) => setDeploymentName(event.target.value)}
                />
              </Field>
              <Field label="Runtime profile">
                <select
                  value={runtimeProfileId}
                  onChange={(event) => setRuntimeProfileId(event.target.value)}
                  className={selectClass}
                >
                  <option value="">Select runtime profile</option>
                  {runtimeProfileOptions.map((profile) => (
                    <option key={profile.id} value={profile.id}>
                      {profile.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Provider account">
                <select
                  value={providerAccountId}
                  onChange={(event) => setProviderAccountId(event.target.value)}
                  className={selectClass}
                >
                  <option value="">Use build default</option>
                  {activeProviderAccounts.map((account) => (
                    <option key={account.id} value={account.id}>
                      {account.name} ({account.provider_key})
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Model alias">
                <select
                  value={modelAliasId}
                  onChange={(event) => setModelAliasId(event.target.value)}
                  className={selectClass}
                >
                  <option value="">Use build default</option>
                  {activeModelAliases.map((alias) => (
                    <option key={alias.id} value={alias.id}>
                      {alias.display_name || alias.alias_key}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
          </Section>

          <Section
            icon={<Play className="size-4" />}
            title="Evaluation"
            meta={selectedVersionId ? "Runnable pack selected" : "Needs pack"}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Challenge pack">
                <select
                  value={selectedPackId}
                  onChange={(event) => {
                    setSelectedPackId(event.target.value);
                    setSelectedVersionId("");
                    setInputSetId("");
                    setSelectedRegressionSuiteIds([]);
                  }}
                  className={selectClass}
                >
                  <option value="">Select challenge pack</option>
                  {challengePacks.map((pack) => (
                    <option key={pack.id} value={pack.id}>
                      {pack.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Runnable version">
                <select
                  value={selectedVersionId}
                  onChange={(event) => setSelectedVersionId(event.target.value)}
                  className={selectClass}
                >
                  <option value="">Select version</option>
                  {runnableVersions.map((version) => (
                    <option key={version.id} value={version.id}>
                      v{version.version_number}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Input set">
                <select
                  value={inputSetId}
                  onChange={(event) => setInputSetId(event.target.value)}
                  className={selectClass}
                  disabled={loadingInputSets}
                >
                  <option value="">
                    {loadingInputSets ? "Loading..." : "Default input set"}
                  </option>
                  {inputSets.map((inputSet) => (
                    <option key={inputSet.id} value={inputSet.id}>
                      {inputSet.name || inputSet.input_key}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <div className="mt-4">
              <div className="mb-2 text-sm font-medium">Regression suites</div>
              {activeRegressionSuites.length === 0 ? (
                <p className="rounded-lg border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                  No active compatible regression suites.
                </p>
              ) : (
                <div className="grid gap-2 md:grid-cols-2">
                  {activeRegressionSuites.map((suite) => {
                    const checked = selectedRegressionSuiteIds.includes(suite.id);
                    return (
                      <label
                        key={suite.id}
                        className={cn(
                          "flex cursor-pointer items-start gap-3 rounded-lg border border-border px-3 py-3 text-sm",
                          checked && "border-foreground/30 bg-muted/40",
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => {
                            setSelectedRegressionSuiteIds((current) =>
                              current.includes(suite.id)
                                ? current.filter((id) => id !== suite.id)
                                : [...current, suite.id],
                            );
                          }}
                          className="mt-1"
                        />
                        <span>
                          <span className="block font-medium">{suite.name}</span>
                          <span className="text-xs text-muted-foreground">
                            {suite.case_count} cases · {suite.default_gate_severity}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                </div>
              )}
            </div>
          </Section>

          <Section
            icon={<ShieldCheck className="size-4" />}
            title="Baseline and Gate"
            meta={baselineStrategy === "locked_run" ? "Locked run" : "Deployment"}
          >
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Baseline strategy">
                <select
                  value={baselineStrategy}
                  onChange={(event) =>
                    setBaselineStrategy(event.target.value as CIBaselineStrategy)
                  }
                  className={selectClass}
                >
                  <option value="locked_run">Locked baseline run</option>
                  <option value="deployment">Resolve from deployment history</option>
                </select>
              </Field>
              {baselineStrategy === "locked_run" ? (
                <>
                  <Field label="Baseline run">
                    <select
                      value={
                        completedRuns.some((run) => run.id === baselineRunId)
                          ? baselineRunId
                          : ""
                      }
                      onChange={(event) => {
                        setBaselineRunIdManualOverride(false);
                        setBaselineRunId(event.target.value);
                      }}
                      className={selectClass}
                    >
                      <option value="">Select completed run</option>
                      {completedRuns.map((run) => (
                        <option key={run.id} value={run.id}>
                          {run.name || run.id}
                        </option>
                      ))}
                    </select>
                  </Field>
                  <Field label="Baseline run ID">
                    <Input
                      value={baselineRunId}
                      onChange={(event) => {
                        setBaselineRunIdManualOverride(true);
                        setBaselineRunId(event.target.value);
                      }}
                      placeholder="run_xxx"
                    />
                  </Field>
                  <Field label="Baseline run agent">
                    <select
                      value={baselineRunAgentId}
                      onChange={(event) =>
                        setBaselineRunAgentId(event.target.value)
                      }
                      className={selectClass}
                      disabled={loadingRunAgents}
                    >
                      <option value="">
                        {loadingRunAgents ? "Loading..." : "Auto-resolve"}
                      </option>
                      {runAgents.map((agent) => (
                        <option key={agent.id} value={agent.id}>
                          {agent.label || agent.id}
                        </option>
                      ))}
                    </select>
                  </Field>
                </>
              ) : (
                <Field label="Baseline deployment">
                  <select
                    value={baselineDeploymentId}
                    onChange={(event) => setBaselineDeploymentId(event.target.value)}
                    className={selectClass}
                  >
                    <option value="">Select deployment</option>
                    {activeDeployments.map((deployment) => (
                      <option key={deployment.id} value={deployment.id}>
                        {deployment.name}
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <Field label="Baseline refresh">
                <select
                  value={baselineRefresh}
                  onChange={(event) =>
                    setBaselineRefresh(event.target.value as CIBaselineRefresh)
                  }
                  className={selectClass}
                >
                  <option value="manual">manual</option>
                  <option value="propose">propose</option>
                  <option value="auto_on_main">auto_on_main</option>
                </select>
              </Field>
              <Field label="Max baseline age (days)">
                <Input
                  type="number"
                  min={0}
                  value={baselineMaxAgeDays}
                  onChange={(event) =>
                    setBaselineMaxAgeDays(Number(event.target.value))
                  }
                />
              </Field>
              <Field label="Fail on">
                <select
                  value={gateFailOn}
                  onChange={(event) => setGateFailOn(event.target.value as CIGateFailOn)}
                  className={selectClass}
                >
                  <option value="regression">regression</option>
                  <option value="warning">warning</option>
                  <option value="insufficient_evidence">insufficient_evidence</option>
                </select>
              </Field>
              <Field label="Policy file">
                <Input
                  value={gatePolicyFile}
                  onChange={(event) => setGatePolicyFile(event.target.value)}
                  placeholder="Optional"
                />
              </Field>
              <Field label="Regression promotion">
                <select
                  value={regressionPromotion}
                  onChange={(event) =>
                    setRegressionPromotion(event.target.value as CIRegressionPromotion)
                  }
                  className={selectClass}
                >
                  <option value="disabled">disabled</option>
                  <option value="proposed">proposed</option>
                  <option value="auto_on_main">auto_on_main</option>
                </select>
              </Field>
            </div>
          </Section>
        </div>

        <aside className="space-y-6 xl:sticky xl:top-0 xl:self-start">
          <StatusPanel
            tone={readiness.ready ? "success" : "warn"}
            title={readiness.ready ? "Ready to add to a repo" : "Setup needs attention"}
            body={
              readiness.ready
                ? "The generated files have the required AgentClash CI fields."
                : readiness.blockers.join(" ")
            }
          />

          {setupPRResult?.pull_request ? (
            <StatusPanel
              tone="success"
              title={`Setup PR #${setupPRResult.pull_request.number} created`}
              body={`Opened ${setupPRResult.branch} against ${setupPRResult.base_branch}.`}
              action={
                <a
                  href={setupPRResult.pull_request.html_url}
                  target="_blank"
                  rel="noreferrer"
                  className="text-sm font-medium underline underline-offset-4"
                >
                  View pull request
                </a>
              }
            />
          ) : null}

          {setupPRConflicts.length > 0 ? (
            <StatusPanel
              tone="warn"
              title="Generated files already exist"
              body={`AgentClash found ${setupPRConflicts.length} target file(s) on ${config.defaultBranch}. Review them before replacing repo CI configuration.`}
              action={
                <div className="space-y-3">
                  <ul className="space-y-1 text-xs text-muted-foreground">
                    {setupPRConflicts.map((conflict) => (
                      <li key={conflict.path}>
                        <code>{conflict.path}</code>
                      </li>
                    ))}
                  </ul>
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={overwriteExisting}
                      onChange={(event) => setOverwriteExisting(event.target.checked)}
                    />
                    Overwrite these files in a setup PR
                  </label>
                </div>
              }
            />
          ) : null}

          <div className="rounded-lg border border-border">
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <div>
                <h2 className="text-sm font-semibold">Generated files</h2>
                <p className="text-xs text-muted-foreground">
                  Copy these into the selected repository.
                </p>
              </div>
              <Badge variant="outline">{readiness.ready ? "valid shape" : "draft"}</Badge>
            </div>
            <Tabs defaultValue="manifest" className="p-4">
              <TabsList>
                <TabsTrigger value="manifest">ci.yaml</TabsTrigger>
                <TabsTrigger value="workflow">workflow</TabsTrigger>
                <TabsTrigger value="review">review</TabsTrigger>
              </TabsList>
              <TabsContent value="manifest" className="pt-4">
                <CodePreview
                  title={config.manifestPath}
                  value={manifest}
                  onCopy={() => copyText(manifest, "Manifest copied")}
                />
              </TabsContent>
              <TabsContent value="workflow" className="pt-4">
                <CodePreview
                  title={config.workflowPath}
                  value={workflow}
                  onCopy={() => copyText(workflow, "Workflow copied")}
                />
              </TabsContent>
              <TabsContent value="review" className="pt-4">
                <ReviewChecklist
                  ready={readiness.ready}
                  blockers={readiness.blockers}
                  config={config}
                />
              </TabsContent>
            </Tabs>
          </div>

          <div className="rounded-lg border border-border p-4">
            <h2 className="text-sm font-semibold">PR behavior</h2>
            <div className="mt-3 grid gap-3 text-sm text-muted-foreground">
              <BehaviorItem title="Check">
                The workflow fails when `agentclash ci run` returns a blocking
                gate verdict.
              </BehaviorItem>
              <BehaviorItem title="Comment">
                The action posts one sticky PR comment with verdict, score deltas,
                run links, comparison links, replay links, and regression tracking.
              </BehaviorItem>
              <BehaviorItem title="Artifacts">
                Result JSON and AgentClash artifact JSON files are uploaded on
                every matched run.
              </BehaviorItem>
              <BehaviorItem title="Regression tracking">
                Failed CI cases are recorded as proposed regression work unless
                auto_on_main promotion is selected for default-branch runs.
              </BehaviorItem>
            </div>
            <div className="mt-4">
              <Link
                href={`/workspaces/${workspaceId}/regression-suites`}
                className="text-sm font-medium underline underline-offset-4"
              >
                Review regression suites
              </Link>
            </div>
          </div>

          <div className="rounded-lg border border-border p-4">
            <h2 className="text-sm font-semibold">Missing resources</h2>
            <ResourceLinks workspaceId={workspaceId} />
          </div>
        </aside>
      </div>
    </div>
  );
}

function Section({
  icon,
  title,
  meta,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  meta?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-border">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <h2 className="flex items-center gap-2 text-sm font-semibold">
          {icon}
          {title}
        </h2>
        {meta ? (
          <span className="text-xs text-muted-foreground">{meta}</span>
        ) : null}
      </div>
      <div className="p-4">{children}</div>
    </section>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium">{label}</span>
      {children}
    </label>
  );
}

function ReadinessBadge({
  ready,
  loading,
}: {
  ready: boolean;
  loading: boolean;
}) {
  if (loading) {
    return (
      <Badge variant="outline">
        <Loader2 className="animate-spin" />
        loading
      </Badge>
    );
  }
  return (
    <Badge variant={ready ? "default" : "secondary"}>
      {ready ? <CheckCircle2 /> : <TriangleAlert />}
      {ready ? "ready" : "draft"}
    </Badge>
  );
}

function StatusPanel({
  tone,
  title,
  body,
  action,
}: {
  tone: "success" | "warn" | "danger";
  title: string;
  body: string;
  action?: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "rounded-lg border p-4",
        tone === "success" && "border-emerald-500/20 bg-emerald-500/5",
        tone === "warn" && "border-amber-500/20 bg-amber-500/5",
        tone === "danger" && "border-destructive/20 bg-destructive/5",
      )}
    >
      <div className="flex items-start gap-3">
        {tone === "success" ? (
          <CheckCircle2 className="mt-0.5 size-4 text-emerald-400" />
        ) : (
          <TriangleAlert
            className={cn(
              "mt-0.5 size-4",
              tone === "danger" ? "text-destructive" : "text-amber-400",
            )}
          />
        )}
        <div>
          <h2 className="text-sm font-semibold">{title}</h2>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">{body}</p>
          {action ? <div className="mt-3">{action}</div> : null}
        </div>
      </div>
    </div>
  );
}

function CodePreview({
  title,
  value,
  onCopy,
}: {
  title: string;
  value: string;
  onCopy: () => void;
}) {
  return (
    <div className="overflow-hidden rounded-lg border border-border">
      <div className="flex items-center justify-between border-b border-border bg-muted/30 px-3 py-2">
        <code className="text-xs text-muted-foreground">{title}</code>
        <Button variant="outline" size="xs" onClick={onCopy}>
          <Clipboard data-icon="inline-start" className="size-3" />
          Copy
        </Button>
      </div>
      <pre className="max-h-[520px] overflow-auto bg-background p-3 text-xs leading-5">
        <code>{value}</code>
      </pre>
    </div>
  );
}

function ReviewChecklist({
  ready,
  blockers,
  config,
}: {
  ready: boolean;
  blockers: string[];
  config: CISetupConfig;
}) {
  const items = [
    ["Manifest", config.manifestPath],
    ["Workflow", config.workflowPath],
    ["Repository", config.repositoryFullName || "not selected"],
    ["Default branch", config.defaultBranch || "not selected"],
    ["Token secret", config.tokenSecretName],
    ["Workspace secret", config.workspaceSecretName],
    ["Trigger paths", `${config.triggerPaths.length}`],
    ["Force labels", `${config.triggerLabels.length}`],
    ["Regression suites", `${config.regressionSuiteIds.length}`],
  ];
  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-border">
        {items.map(([label, value]) => (
          <div
            key={label}
            className="flex items-center justify-between border-b border-border px-3 py-2 text-sm last:border-b-0"
          >
            <span className="text-muted-foreground">{label}</span>
            <code className="text-right text-xs">{value}</code>
          </div>
        ))}
      </div>
      {!ready ? (
        <div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3">
          <div className="text-sm font-medium">Blockers</div>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-sm text-muted-foreground">
            {blockers.map((blocker) => (
              <li key={blocker}>{blocker}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function BehaviorItem({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid grid-cols-[96px_1fr] gap-3">
      <span className="text-xs font-semibold uppercase tracking-[0.12em] text-muted-foreground">
        {title}
      </span>
      <span>{children}</span>
    </div>
  );
}

function ResourceLinks({ workspaceId }: { workspaceId: string }) {
  const links = [
    ["Builds", `/workspaces/${workspaceId}/builds`],
    ["Runtime profiles", `/workspaces/${workspaceId}/runtime-profiles`],
    ["Challenge packs", `/workspaces/${workspaceId}/challenge-packs`],
    ["Runs", `/workspaces/${workspaceId}/runs`],
    ["Regression suites", `/workspaces/${workspaceId}/regression-suites`],
    ["Provider accounts", `/workspaces/${workspaceId}/provider-accounts`],
    ["Model aliases", `/workspaces/${workspaceId}/model-aliases`],
  ];
  return (
    <div className="mt-3 flex flex-wrap gap-2">
      {links.map(([label, href]) => (
        <Link
          key={href}
          href={href}
          className="rounded-md border border-border px-2.5 py-1.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          {label}
        </Link>
      ))}
    </div>
  );
}

type StoredCIProfileConfig = {
  schemaVersion?: unknown;
  selectedRepositoryKey?: unknown;
  selectedPackId?: unknown;
  selectedRegressionSuiteIds?: unknown;
  triggerPaths?: unknown;
  triggerLabels?: unknown;
  workflowPath?: unknown;
  manifestPath?: unknown;
  agentSpecPath?: unknown;
  repositoryFullName?: unknown;
  defaultBranch?: unknown;
  agentBuildId?: unknown;
  runtimeProfileId?: unknown;
  providerAccountId?: unknown;
  modelAliasId?: unknown;
  deploymentName?: unknown;
  selectedVersionId?: unknown;
  inputSetId?: unknown;
  baselineStrategy?: unknown;
  baselineRunId?: unknown;
  baselineRunAgentId?: unknown;
  baselineDeploymentId?: unknown;
  baselineRefresh?: unknown;
  baselineMaxAgeDays?: unknown;
  gateFailOn?: unknown;
  gatePolicyFile?: unknown;
  regressionPromotion?: unknown;
};

function buildProfileConfig(config: {
  selectedRepositoryKey: string;
  selectedPackId: string;
  selectedRegressionSuiteIds: string[];
  triggerPathsText: string;
  triggerLabelsText: string;
  workflowPath: string;
  manifestPath: string;
  agentSpecPath: string;
  repositoryFullName: string;
  defaultBranch: string;
  agentBuildId: string;
  runtimeProfileId: string;
  providerAccountId: string;
  modelAliasId: string;
  deploymentName: string;
  selectedVersionId: string;
  inputSetId: string;
  baselineStrategy: CIBaselineStrategy;
  baselineRunId: string;
  baselineRunAgentId: string;
  baselineDeploymentId: string;
  baselineRefresh: CIBaselineRefresh;
  baselineMaxAgeDays: number;
  gateFailOn: CIGateFailOn;
  gatePolicyFile: string;
  regressionPromotion: CIRegressionPromotion;
}): StoredCIProfileConfig {
  return {
    schemaVersion: 1,
    selectedRepositoryKey: config.selectedRepositoryKey,
    selectedPackId: config.selectedPackId,
    selectedRegressionSuiteIds: config.selectedRegressionSuiteIds,
    triggerPaths: splitLines(config.triggerPathsText),
    triggerLabels: splitLines(config.triggerLabelsText),
    workflowPath: config.workflowPath,
    manifestPath: config.manifestPath,
    agentSpecPath: config.agentSpecPath,
    repositoryFullName: config.repositoryFullName,
    defaultBranch: config.defaultBranch,
    agentBuildId: config.agentBuildId,
    runtimeProfileId: config.runtimeProfileId,
    providerAccountId: config.providerAccountId,
    modelAliasId: config.modelAliasId,
    deploymentName: config.deploymentName,
    selectedVersionId: config.selectedVersionId,
    inputSetId: config.inputSetId,
    baselineStrategy: config.baselineStrategy,
    baselineRunId: config.baselineRunId,
    baselineRunAgentId: config.baselineRunAgentId,
    baselineDeploymentId: config.baselineDeploymentId,
    baselineRefresh: config.baselineRefresh,
    baselineMaxAgeDays: config.baselineMaxAgeDays,
    gateFailOn: config.gateFailOn,
    gatePolicyFile: config.gatePolicyFile,
    regressionPromotion: config.regressionPromotion,
  };
}

function normalizeProfileConfig(config: unknown): StoredCIProfileConfig {
  return config && typeof config === "object" && !Array.isArray(config)
    ? (config as StoredCIProfileConfig)
    : {};
}

function stringValue(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}

function numberValue(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function stringArrayValue(value: unknown, fallback: string[]): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : fallback;
}

function splitLines(value: string): string[] {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function gitHubRepositoryKey(repo: GitHubRepository): string {
  return `${repo.github_installation_id}:${repo.github_repository_id}`;
}

async function copyText(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value);
    toast.success(message);
  } catch {
    toast.error("Clipboard is unavailable");
  }
}

function downloadText(path: string, value: string) {
  const filename = path.split("/").filter(Boolean).at(-1) || "agentclash-ci.yaml";
  const url = URL.createObjectURL(new Blob([value], { type: "text/yaml" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}
