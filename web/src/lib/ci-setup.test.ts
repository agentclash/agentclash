import { describe, expect, it } from "vitest";
import {
  ciSetupReadiness,
  defaultCISetupConfig,
  generateAgentClashCIManifest,
  generateAgentClashGitHubWorkflow,
  type CISetupConfig,
} from "./ci-setup";

const completeConfig: CISetupConfig = {
  ...defaultCISetupConfig,
  repositoryFullName: "acme/support-agent",
  agentBuildId: "build-1",
  runtimeProfileId: "runtime-1",
  providerAccountId: "provider-1",
  model: "model-1",
  challengePackVersionId: "pack-version-1",
  inputSetId: "input-set-1",
  regressionSuiteIds: ["suite-1"],
  regressionCaseIds: ["case-1"],
  baselineRunId: "baseline-run-1",
  baselineRunAgentId: "baseline-agent-1",
};

describe("generateAgentClashCIManifest", () => {
  it("generates the existing CLI manifest shape", () => {
    expect(generateAgentClashCIManifest(completeConfig)).toBe(`version: 1
trigger:
  paths:
    - ".agentclash/agent.json"
    - "prompts/**"
    - "tools/**"
  labels:
    - "agentclash/eval"
candidate:
  build:
    agent_build_id: "build-1"
    spec_file: ".agentclash/agent.json"
  deployment:
    name: "pr-candidate"
    runtime_profile_id: "runtime-1"
    provider_account_id: "provider-1"
    model: "model-1"
evaluation:
  challenge_pack_version_id: "pack-version-1"
  input_set_id: "input-set-1"
  regression_suites:
    - "suite-1"
  regression_cases:
    - "case-1"
baseline:
  run_id: "baseline-run-1"
  run_agent_id: "baseline-agent-1"
  refresh: "manual"
  max_age_days: 30
gate:
  fail_on: "regression"
regressions:
  promote_failures: "proposed"
`);
  });

  it("omits optional fields when unset", () => {
    const yaml = generateAgentClashCIManifest({
      ...completeConfig,
      providerAccountId: "",
      model: "",
      inputSetId: "",
      regressionSuiteIds: [],
      regressionCaseIds: [],
      baselineRunAgentId: "",
      baselineMaxAgeDays: 0,
      triggerLabels: [],
    });

    expect(yaml).not.toContain("provider_account_id");
    expect(yaml).not.toContain("model:");
    expect(yaml).not.toContain("input_set_id");
    expect(yaml).not.toContain("regression_suites");
    expect(yaml).not.toContain("regression_cases");
    expect(yaml).not.toContain("run_agent_id");
    expect(yaml).not.toContain("max_age_days");
    expect(yaml).not.toContain("labels:");
  });

  it("supports deployment-derived baselines", () => {
    const yaml = generateAgentClashCIManifest({
      ...completeConfig,
      baselineStrategy: "deployment",
      baselineRunId: "",
      baselineRunAgentId: "",
      baselineDeploymentId: "deployment-1",
      baselineRefresh: "propose",
    });

    expect(yaml).toContain('  deployment_id: "deployment-1"');
    expect(yaml).toContain('  refresh: "propose"');
    expect(yaml).not.toContain("run_id:");
  });

  it("quotes special YAML characters safely", () => {
    const yaml = generateAgentClashCIManifest({
      ...completeConfig,
      triggerPaths: ["**/*.prompt.yml", "agents:prod/**"],
      triggerLabels: ["agentclash/eval:force"],
      deploymentName: "candidate: pr #1",
    });

    expect(yaml).toContain('    - "**/*.prompt.yml"');
    expect(yaml).toContain('    - "agents:prod/**"');
    expect(yaml).toContain('    - "agentclash/eval:force"');
    expect(yaml).toContain('    name: "candidate: pr #1"');
  });
});

describe("generateAgentClashGitHubWorkflow", () => {
  it("generates a GitHub Actions workflow around the AgentClash composite action", () => {
    const workflow = generateAgentClashGitHubWorkflow(completeConfig);

    expect(workflow).toContain("uses: agentclash/agentclash/.github/actions/agentclash-ci@main");
    expect(workflow).toContain("contents: read");
    expect(workflow).toContain("pull-requests: write");
    expect(workflow).toContain("token: ${{ secrets.AGENTCLASH_TOKEN }}");
    expect(workflow).toContain("workspace: ${{ secrets.AGENTCLASH_WORKSPACE }}");
    expect(workflow).toContain('manifest: ".agentclash/ci.yaml"');
    expect(workflow).toContain("types: [opened, synchronize, reopened, labeled, unlabeled]");
    expect(workflow).toContain(
      "labels: \"${{ github.event_name == 'pull_request' && join(github.event.pull_request.labels.*.name, ',') || '' }}\"",
    );
    expect(workflow).toContain('default-branch: "main"');
    expect(workflow).toContain("# AgentClash repository: acme/support-agent");
    expect(workflow).toContain("uses: actions/upload-artifact@v4");
    expect(workflow).toContain("${{ steps.agentclash.outputs.result-file }}");
  });

  it("leaves path and label matching to the AgentClash action", () => {
    const workflow = generateAgentClashGitHubWorkflow({
      ...completeConfig,
      triggerPaths: [".agentclash/ci.yaml", ".agentclash/agent.json"],
    });

    expect(workflow).not.toContain("    paths:");
    expect(workflow).toContain("skip-if-unmatched");
    expect(workflow).toContain("labels:");
  });
});

describe("ciSetupReadiness", () => {
  it("passes complete configuration", () => {
    expect(ciSetupReadiness(completeConfig)).toEqual({
      ready: true,
      blockers: [],
    });
  });

  it("reports human-readable blockers", () => {
    expect(
      ciSetupReadiness({
        ...defaultCISetupConfig,
        triggerPaths: [],
      }),
    ).toEqual({
      ready: false,
      blockers: [
        "Select an agent build for candidate versions.",
        "Select a runtime profile for CI deployments.",
        "Select a runnable challenge pack version.",
        "Add at least one watched path glob.",
        "Select a locked baseline run.",
      ],
    });
  });
});
