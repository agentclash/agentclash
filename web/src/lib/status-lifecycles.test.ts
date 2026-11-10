import { describe, expect, it } from "vitest";

import type {
  EvalSessionStatus,
  EvalSetStatus,
  RegressionCaseStatus,
  RegressionSuiteStatus,
  RunAgentStatus,
  RunStatus,
} from "@/lib/api/types";

import generated from "./generated/status-lifecycles.json";
import {
  LIFECYCLE_UNIONS_MATCH_SERVER,
  isTerminal,
  nextStatuses,
  nonTerminalStates,
  states,
  terminalStates,
  type LifecycleResource,
} from "./status-lifecycles";

const lifecycles = generated.lifecycles as Record<
  LifecycleResource,
  {
    states: string[];
    terminal: string[];
    transitions: Record<string, string[]>;
  }
>;
const resources = Object.keys(lifecycles) as LifecycleResource[];

// The members of each union in api/types.ts. Test files are not type checked,
// so `tsc` compares the unions with the generated JSON in
// status-lifecycles.ts (LIFECYCLE_UNIONS_MATCH_SERVER). These records repeat
// the unions so that a server change without a web change also fails here.
const UNION_MEMBERS: { [R in LifecycleResource]: string[] } = {
  run: Object.keys({
    draft: true,
    queued: true,
    provisioning: true,
    running: true,
    scoring: true,
    completed: true,
    failed: true,
    cancelled: true,
  } satisfies Record<RunStatus, true>),
  run_agent: Object.keys({
    queued: true,
    ready: true,
    executing: true,
    evaluating: true,
    completed: true,
    failed: true,
  } satisfies Record<RunAgentStatus, true>),
  eval_session: Object.keys({
    queued: true,
    running: true,
    aggregating: true,
    completed: true,
    failed: true,
    cancelled: true,
  } satisfies Record<EvalSessionStatus, true>),
  eval_set: Object.keys({
    queued: true,
    expanding: true,
    running: true,
    aggregating: true,
    completed: true,
    failed: true,
    cancelled: true,
    budget_exceeded: true,
  } satisfies Record<EvalSetStatus, true>),
  regression_suite: Object.keys({
    active: true,
    archived: true,
  } satisfies Record<RegressionSuiteStatus, true>),
  regression_case: Object.keys({
    proposed: true,
    active: true,
    muted: true,
    archived: true,
    rejected: true,
  } satisfies Record<RegressionCaseStatus, true>),
};

const sorted = (values: readonly string[]) => [...values].sort();

describe("generated status lifecycles", () => {
  it("publishes every resource that the web app types", () => {
    expect(sorted(resources)).toEqual(sorted(Object.keys(UNION_MEMBERS)));
    expect(sorted(Object.keys(LIFECYCLE_UNIONS_MATCH_SERVER))).toEqual(
      sorted(resources),
    );
    expect(Object.values(LIFECYCLE_UNIONS_MATCH_SERVER).every(Boolean)).toBe(
      true,
    );
  });

  it.each(resources)("%s: the union in api/types.ts equals the state list", (resource) => {
    expect(sorted(states(resource))).toEqual(sorted(UNION_MEMBERS[resource]));
  });

  it.each(resources)("%s: the state list equals the transition keys", (resource) => {
    const lifecycle = lifecycles[resource];
    expect(new Set(lifecycle.states).size).toBe(lifecycle.states.length);
    // tsc checks the unions against the transition keys, so this ties the
    // unions to the state list too.
    expect(sorted(lifecycle.states)).toEqual(
      sorted(Object.keys(lifecycle.transitions)),
    );
  });

  it.each(resources)("%s: terminal states are the states with no next status", (resource) => {
    const lifecycle = lifecycles[resource];
    expect(lifecycle.terminal).toEqual(
      lifecycle.states.filter((s) => lifecycle.transitions[s].length === 0),
    );
    for (const targets of Object.values(lifecycle.transitions)) {
      for (const target of targets) expect(lifecycle.states).toContain(target);
    }
  });
});

describe("status lifecycle helpers", () => {
  it("reads states and terminal states in lifecycle order", () => {
    expect(states("run")).toEqual([
      "draft",
      "queued",
      "provisioning",
      "running",
      "scoring",
      "completed",
      "failed",
      "cancelled",
    ]);
    expect(terminalStates("run_agent")).toEqual(["completed", "failed"]);
    expect(terminalStates("regression_suite")).toEqual([]);
    expect(nonTerminalStates("eval_session")).toEqual([
      "queued",
      "running",
      "aggregating",
    ]);
  });

  it("knows terminal statuses and ignores unknown ones", () => {
    expect(isTerminal("eval_set", "budget_exceeded")).toBe(true);
    expect(isTerminal("eval_set", "running")).toBe(false);
    expect(isTerminal("run", "pending")).toBe(false);
  });

  it("returns the legal next statuses", () => {
    expect(nextStatuses("regression_case", "proposed")).toEqual([
      "active",
      "archived",
      "rejected",
    ]);
    expect(nextStatuses("regression_case", "archived")).toEqual([]);
    expect(nextStatuses("run", "unknown")).toEqual([]);
    expect(nextStatuses("run", "constructor")).toEqual([]);
  });
});
