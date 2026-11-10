import type {
  EvalSessionStatus,
  EvalSetStatus,
  RegressionCaseStatus,
  RegressionSuiteStatus,
  RunAgentStatus,
  RunStatus,
} from "@/lib/api/types";

import generated from "./generated/status-lifecycles.json";

/**
 * Status lifecycles from the server transition tables in runtime/domain.
 *
 * generated/status-lifecycles.json is generated from domain.Lifecycles(). Do
 * not edit it. Regenerate it with:
 *
 *   cd runtime && go test ./domain -run TestWebStatusLifecyclesMatchDomain -update
 *
 * Read status sets from this module. Do not keep a copy of a status list in a
 * component. See docs/state-machines.md.
 */

type GeneratedLifecycles = (typeof generated)["lifecycles"];

export type LifecycleResource = keyof GeneratedLifecycles;

/** The union type in api/types.ts for each lifecycle resource. */
export interface LifecycleStatus {
  run: RunStatus;
  run_agent: RunAgentStatus;
  eval_session: EvalSessionStatus;
  eval_set: EvalSetStatus;
  regression_suite: RegressionSuiteStatus;
  regression_case: RegressionCaseStatus;
}

export type StatusOf<R extends LifecycleResource> = LifecycleStatus[R];

// The JSON import keeps the keys of each transition map as literal types, so
// the compiler can compare them with the unions in api/types.ts.
type ServerStatus<R extends LifecycleResource> =
  keyof GeneratedLifecycles[R]["transitions"] & string;

type Same<A, B> = [A] extends [B] ? ([B] extends [A] ? true : false) : false;

type UnionMatchesServer<R extends LifecycleResource> =
  Same<StatusOf<R>, ServerStatus<R>> extends true
    ? true
    : `The ${R} status union in api/types.ts does not match the server lifecycle. Update the union.`;

/**
 * Compile-time check: type checking fails here when a status union in
 * api/types.ts and the generated server lifecycle name different statuses.
 * status-lifecycles.test.ts checks the state lists at runtime.
 */
export const LIFECYCLE_UNIONS_MATCH_SERVER: {
  [R in LifecycleResource]: UnionMatchesServer<R>;
} = {
  run: true,
  run_agent: true,
  eval_session: true,
  eval_set: true,
  regression_suite: true,
  regression_case: true,
};

interface Lifecycle<S extends string> {
  readonly states: readonly S[];
  readonly terminal: readonly S[];
  readonly transitions: Readonly<Record<S, readonly S[]>>;
}

// The check above ties the transition keys to each union, and the test ties
// the state lists to the transition keys, so this cast is safe.
const LIFECYCLES = generated.lifecycles as unknown as {
  readonly [R in LifecycleResource]: Lifecycle<StatusOf<R>>;
};

/** Every status of the resource, in lifecycle order. */
export function states<R extends LifecycleResource>(
  resource: R,
): readonly StatusOf<R>[] {
  return LIFECYCLES[resource].states;
}

/** The statuses that have no next status. A poll loop stops on these. */
export function terminalStates<R extends LifecycleResource>(
  resource: R,
): readonly StatusOf<R>[] {
  return LIFECYCLES[resource].terminal;
}

/** Every status that is not terminal, in lifecycle order. */
export function nonTerminalStates<R extends LifecycleResource>(
  resource: R,
): StatusOf<R>[] {
  return states(resource).filter((status) => !isTerminal(resource, status));
}

/** Whether the status is terminal. An unknown status is not terminal. */
export function isTerminal<R extends LifecycleResource>(
  resource: R,
  status: string,
): boolean {
  return (terminalStates(resource) as readonly string[]).includes(status);
}

/**
 * The legal next statuses, in lifecycle order. A terminal or unknown status
 * has none.
 */
export function nextStatuses<R extends LifecycleResource>(
  resource: R,
  status: string,
): readonly StatusOf<R>[] {
  const transitions: Readonly<Record<string, readonly StatusOf<R>[]>> =
    LIFECYCLES[resource].transitions;
  return Object.prototype.hasOwnProperty.call(transitions, status)
    ? transitions[status]
    : [];
}
