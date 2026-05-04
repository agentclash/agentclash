import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { describe, expect, it, vi } from "vitest";

import type {
  ChallengePack,
  RegressionCase,
  RegressionSuite,
} from "@/lib/api/types";

import { SuiteDetailClient } from "./suite-detail-client";
import { CaseDetailClient } from "./cases/[caseId]/case-detail-client";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) =>
    React.createElement("a", { href, ...props }, children),
}));

vi.mock("@/components/ui/page-header", () => ({
  PageHeader: ({
    title,
    actions,
  }: {
    title: string;
    actions?: React.ReactNode;
  }) => (
    <header>
      <h1>{title}</h1>
      {actions}
    </header>
  ),
}));

vi.mock("./edit-suite-dialog", () => ({
  EditSuiteDialog: () => null,
}));

vi.mock("./suite-run-history", () => ({
  SuiteRunHistory: () => <div>run-history</div>,
}));

vi.mock("./cases/[caseId]/edit-case-dialog", () => ({
  EditCaseDialog: () => null,
}));

const suite: RegressionSuite = {
  id: "suite-1",
  workspace_id: "ws-1",
  source_challenge_pack_id: "pack-1",
  name: "Critical regressions",
  description: "",
  status: "active",
  source_mode: "derived_only",
  default_gate_severity: "warning",
  case_count: 1,
  created_by_user_id: "user-1",
  created_at: "2026-04-22T12:00:00Z",
  updated_at: "2026-04-22T12:00:00Z",
};

const sourcePack: ChallengePack = {
  id: "pack-1",
  name: "Support Tickets",
  description: "",
  slug: "support-tickets",
  versions: [],
  created_at: "2026-04-22T12:00:00Z",
  updated_at: "2026-04-22T12:00:00Z",
};

function makeCase(overrides: Partial<RegressionCase> = {}): RegressionCase {
  return {
    id: "case-1",
    suite_id: "suite-1",
    workspace_id: "ws-1",
    title: "Policy regression",
    description: "",
    status: "active",
    severity: "blocking",
    promotion_mode: "full_executable",
    source_run_id: "run-1",
    source_run_agent_id: "agent-1",
    source_challenge_pack_version_id: "version-1",
    source_challenge_identity_id: "identity-1",
    source_challenge_key: "ticket-1",
    source_case_key: "case-a",
    source_item_key: "prompt.txt",
    source_failure_fingerprint: "frf-test-fingerprint",
    source_failure_cluster_key: "frc-test-cluster",
    evidence_tier: "native_structured",
    failure_class: "policy_violation",
    failure_summary: "The agent violated policy.",
    payload_snapshot: {},
    expected_contract: {},
    validator_overrides: null,
    metadata: {
      source_failure_fingerprint: "frf-test-fingerprint",
      source_failure_cluster_key: "frc-test-cluster",
    },
    validation: {
      status: "reproducing",
      run_count: 5,
      failure_count: 3,
      pass_count: 2,
      reproduction_rate: 0.6,
      reproduction_threshold: 0.6,
      required_runs: 5,
      remaining_runs: 0,
      last_outcome: "pass",
      last_validated_at: "2026-04-22T12:30:00Z",
      recommended_action:
        "Failure reproduces at or above threshold; keep this case active in CI gates.",
    },
    created_at: "2026-04-22T12:00:00Z",
    updated_at: "2026-04-22T12:00:00Z",
    ...overrides,
  };
}

function render(element: React.ReactElement) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  act(() => {
    root.render(element);
  });
  return {
    container,
    cleanup() {
      act(() => {
        root.unmount();
      });
      container.remove();
    },
  };
}

describe("regression provenance UI", () => {
  it("shows failure provenance in the suite case list", () => {
    const view = render(
      <SuiteDetailClient
        workspaceId="ws-1"
        suite={suite}
        cases={[makeCase()]}
        sourcePack={sourcePack}
      />,
    );
    try {
      expect(view.container.textContent).toContain("ticket-1");
      expect(view.container.textContent).toContain("frc-test-cluster");
      expect(view.container.textContent).toContain("reproducing");
      expect(view.container.textContent).toContain("60% repro");
    } finally {
      view.cleanup();
    }
  });

  it("shows failure provenance in the case detail view", () => {
    const view = render(
      <CaseDetailClient
        workspaceId="ws-1"
        suite={suite}
        regressionCase={makeCase()}
      />,
    );
    try {
      expect(view.container.textContent).toContain("Challenge Key");
      expect(view.container.textContent).toContain("ticket-1");
      expect(view.container.textContent).toContain("Failure Cluster");
      expect(view.container.textContent).toContain("frc-test-cluster");
      expect(view.container.textContent).toContain("Failure Fingerprint");
      expect(view.container.textContent).toContain("frf-test-fingerprint");
      expect(view.container.textContent).toContain("Validation");
      expect(view.container.textContent).toContain("3 fail");
      expect(view.container.textContent).toContain("60% / 60%");
    } finally {
      view.cleanup();
    }
  });
});
