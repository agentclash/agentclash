export type CIBaselineStrategy = "locked_run" | "deployment";
export type CIGateFailOn = "regression" | "warning" | "insufficient_evidence";
export type CIRegressionPromotion = "disabled" | "proposed" | "auto_on_main";
export type CIBaselineRefresh = "manual" | "propose" | "auto_on_main";

export interface CISetupConfig {
  repositoryFullName: string;
  defaultBranch: string;
  workflowPath: string;
  manifestPath: string;
  agentSpecPath: string;
  triggerPaths: string[];
  triggerLabels: string[];
  agentBuildId: string;
  runtimeProfileId: string;
  deploymentName: string;
  providerAccountId?: string;
  model?: string;
  challengePackVersionId: string;
  inputSetId?: string;
  regressionSuiteIds: string[];
  regressionCaseIds: string[];
  baselineStrategy: CIBaselineStrategy;
  baselineRunId?: string;
  baselineRunAgentId?: string;
  baselineDeploymentId?: string;
  baselineRefresh: CIBaselineRefresh;
  baselineMaxAgeDays: number;
  gateFailOn: CIGateFailOn;
  gatePolicyFile?: string;
  regressionPromotion: CIRegressionPromotion;
  tokenSecretName: string;
  workspaceSecretName: string;
  appUrl?: string;
  apiUrl?: string;
}

export interface CISetupReadiness {
  ready: boolean;
  blockers: string[];
}

const DEFAULT_WORKFLOW_PATH = ".github/workflows/agentclash.yml";
const DEFAULT_MANIFEST_PATH = ".agentclash/ci.yaml";
const DEFAULT_AGENT_SPEC_PATH = ".agentclash/agent.json";

export const defaultCISetupConfig: CISetupConfig = {
  repositoryFullName: "",
  defaultBranch: "main",
  workflowPath: DEFAULT_WORKFLOW_PATH,
  manifestPath: DEFAULT_MANIFEST_PATH,
  agentSpecPath: DEFAULT_AGENT_SPEC_PATH,
  triggerPaths: [DEFAULT_AGENT_SPEC_PATH, "prompts/**", "tools/**"],
  triggerLabels: ["agentclash/eval"],
  agentBuildId: "",
  runtimeProfileId: "",
  deploymentName: "pr-candidate",
  challengePackVersionId: "",
  regressionSuiteIds: [],
  regressionCaseIds: [],
  baselineStrategy: "locked_run",
  baselineRefresh: "manual",
  baselineMaxAgeDays: 30,
  gateFailOn: "regression",
  regressionPromotion: "proposed",
  tokenSecretName: "AGENTCLASH_TOKEN",
  workspaceSecretName: "AGENTCLASH_WORKSPACE",
};

export function ciSetupReadiness(config: CISetupConfig): CISetupReadiness {
  const blockers: string[] = [];
  const requireValue = (value: string | undefined, message: string) => {
    if (!value?.trim()) blockers.push(message);
  };

  requireValue(config.manifestPath, "Choose where .agentclash/ci.yaml will live.");
  requireValue(config.workflowPath, "Choose where the GitHub workflow will live.");
  requireValue(config.agentSpecPath, "Choose the candidate agent spec file path.");
  requireValue(config.agentBuildId, "Select an agent build for candidate versions.");
  requireValue(config.runtimeProfileId, "Select a runtime profile for CI deployments.");
  requireValue(
    config.challengePackVersionId,
    "Select a runnable challenge pack version.",
  );

  if (nonEmpty(config.triggerPaths).length === 0) {
    blockers.push("Add at least one watched path glob.");
  }

  if (config.baselineStrategy === "locked_run") {
    requireValue(config.baselineRunId, "Select a locked baseline run.");
  } else {
    requireValue(
      config.baselineDeploymentId,
      "Select a baseline deployment to resolve from run history.",
    );
  }

  if (config.baselineMaxAgeDays < 0) {
    blockers.push("Baseline max age must be zero or greater.");
  }

  return { ready: blockers.length === 0, blockers };
}

export function generateAgentClashCIManifest(config: CISetupConfig): string {
  const lines: string[] = [
    "version: 1",
    "trigger:",
    "  paths:",
    ...nonEmpty(config.triggerPaths).map((path) => `    - ${yamlScalar(path)}`),
  ];

  const labels = nonEmpty(config.triggerLabels);
  if (labels.length > 0) {
    lines.push("  labels:");
    lines.push(...labels.map((label) => `    - ${yamlScalar(label)}`));
  }

  lines.push(
    "candidate:",
    "  build:",
    `    agent_build_id: ${yamlScalar(config.agentBuildId)}`,
    `    spec_file: ${yamlScalar(config.agentSpecPath)}`,
    "  deployment:",
  );

  if (config.deploymentName.trim()) {
    lines.push(`    name: ${yamlScalar(config.deploymentName)}`);
  }
  lines.push(`    runtime_profile_id: ${yamlScalar(config.runtimeProfileId)}`);
  if (config.providerAccountId?.trim()) {
    lines.push(
      `    provider_account_id: ${yamlScalar(config.providerAccountId)}`,
    );
  }
  if (config.model?.trim()) {
    lines.push(`    model: ${yamlScalar(config.model)}`);
  }

  lines.push(
    "evaluation:",
    `  challenge_pack_version_id: ${yamlScalar(config.challengePackVersionId)}`,
  );
  if (config.inputSetId?.trim()) {
    lines.push(`  input_set_id: ${yamlScalar(config.inputSetId)}`);
  }
  appendStringList(lines, "  regression_suites:", config.regressionSuiteIds, 4);
  appendStringList(lines, "  regression_cases:", config.regressionCaseIds, 4);

  lines.push("baseline:");
  if (config.baselineStrategy === "locked_run") {
    lines.push(`  run_id: ${yamlScalar(config.baselineRunId ?? "")}`);
    if (config.baselineRunAgentId?.trim()) {
      lines.push(`  run_agent_id: ${yamlScalar(config.baselineRunAgentId)}`);
    }
  } else {
    lines.push(
      `  deployment_id: ${yamlScalar(config.baselineDeploymentId ?? "")}`,
    );
  }
  if (config.baselineRefresh) {
    lines.push(`  refresh: ${yamlScalar(config.baselineRefresh)}`);
  }
  if (config.baselineMaxAgeDays > 0) {
    lines.push(`  max_age_days: ${config.baselineMaxAgeDays}`);
  }

  lines.push("gate:", `  fail_on: ${yamlScalar(config.gateFailOn)}`);
  if (config.gatePolicyFile?.trim()) {
    lines.push(`  policy_file: ${yamlScalar(config.gatePolicyFile)}`);
  }
  lines.push(
    "regressions:",
    `  promote_failures: ${yamlScalar(config.regressionPromotion)}`,
    "",
  );

  return lines.join("\n");
}

export function generateAgentClashGitHubWorkflow(config: CISetupConfig): string {
  const lines = [
    "name: AgentClash CI",
    config.repositoryFullName
      ? `# AgentClash repository: ${config.repositoryFullName.trim()}`
      : "",
    "",
    "on:",
    "  pull_request:",
    "    types: [opened, synchronize, reopened, labeled, unlabeled]",
    "  workflow_dispatch:",
    "",
    "concurrency:",
    "  group: agentclash-${{ github.workflow }}-${{ github.ref }}",
    "  cancel-in-progress: true",
    "",
    "jobs:",
    "  agentclash:",
    "    runs-on: ubuntu-latest",
    "    permissions:",
    "      contents: read",
    "      pull-requests: write",
    "",
    "    steps:",
    "      - uses: actions/checkout@v4",
    "        with:",
    "          fetch-depth: 0",
    "",
    "      - uses: actions/setup-node@v4",
    "        with:",
    "          node-version: \"22\"",
    "",
    "      - id: agentclash",
    "        uses: agentclash/agentclash/.github/actions/agentclash-ci@main",
    "        with:",
    `          token: \${{ secrets.${secretName(config.tokenSecretName)} }}`,
    `          workspace: \${{ secrets.${secretName(config.workspaceSecretName)} }}`,
    `          manifest: ${yamlScalar(config.manifestPath)}`,
    `          labels: "\${{ github.event_name == 'pull_request' && join(github.event.pull_request.labels.*.name, ',') || '' }}"`,
    `          default-branch: ${yamlScalar(config.defaultBranch)}`,
    "          skip-if-unmatched: \"true\"",
  ];

  if (config.apiUrl?.trim()) {
    lines.push(`          api-url: ${yamlScalar(config.apiUrl)}`);
  }
  if (config.appUrl?.trim()) {
    lines.push(`          app-url: ${yamlScalar(config.appUrl)}`);
  }

  lines.push(
    "",
    "      - name: Upload AgentClash gate artifacts",
    "        if: always() && steps.agentclash.outputs['should-run'] == 'true'",
    "        uses: actions/upload-artifact@v4",
    "        with:",
    "          name: agentclash-ci",
    "          path: |",
    "            ${{ steps.agentclash.outputs.result-file }}",
    "            ${{ steps.agentclash.outputs.artifact-dir }}/*.json",
    "",
  );

  return lines.join("\n");
}

function appendStringList(
  lines: string[],
  label: string,
  values: string[],
  indent: number,
) {
  const items = nonEmpty(values);
  if (items.length === 0) return;
  lines.push(label);
  lines.push(...items.map((value) => `${" ".repeat(indent)}- ${yamlScalar(value)}`));
}

function yamlScalar(value: string): string {
  return JSON.stringify(value);
}

function nonEmpty(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

function secretName(value: string): string {
  return value.trim().replace(/[^A-Z0-9_]/gi, "_").toUpperCase();
}
