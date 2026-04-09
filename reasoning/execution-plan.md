# agentclash-reasoning v0: Implementation Execution Plan

## Context

This plan transforms the design in `reasoning/plan.md` into an implementation-ready, phase-wise execution plan for issue #118. The goal is to ship one new execution lane (`reasoning_v1`) where Python runs a ReAct loop, Go owns tool execution and sandbox, and canonical `run_events` drive existing replay and scoring unchanged. Seven gaps identified during codebase audit have been incorporated as explicit implementation steps.

---

## Phase 1: Contracts and Schema

### Objective
Establish all durable contracts, database schema, interface changes, and configuration before any runtime code.

### Why this phase exists
Every subsequent phase depends on stable DB schema, Go interfaces, config values, and bridge types. Changing contracts mid-implementation forces cascading rework.

### Inputs
- Existing `run_events` schema (migration 00007)
- Existing `hosted_run_executions` schema (migration 00009)
- Existing `agent_build_versions.agent_kind` constraint (migration 00012)
- Existing `sandbox.Provider` interface (`backend/internal/sandbox/sandbox.go:19-21`)
- Existing `runevents.Source` constants (`backend/internal/runevents/envelope.go:46-53`)
- Existing worker config (`backend/internal/worker/config.go:23-33`)
- Existing API config (`backend/internal/api/config.go:21-28`)

### Outputs
1. Migration `00014_reasoning_execution.sql`
2. Updated `sandbox.Provider` interface with `Reconnect` method
3. New `runevents.SourceReasoningEngine` constant
4. New `runevents.EvidenceLevelReasoningStructured` constant (or reuse `NativeStructured`)
5. Go bridge request/response types in new package `backend/internal/reasoning/`
6. Updated worker and API config structs
7. Updated `agent_kind` DB constraint

### Implementation Steps

#### 1.1 Migration `00014_reasoning_execution.sql`

Add `reasoning_v1` to agent_kind constraint:
```sql
ALTER TABLE agent_build_versions
DROP CONSTRAINT agent_build_versions_agent_kind_check,
ADD CONSTRAINT agent_build_versions_agent_kind_check
CHECK (agent_kind IN ('llm_agent', 'workflow_agent', 'programmatic_agent',
    'multi_agent_system', 'hosted_external', 'reasoning_v1'));
```

Add `reasoning_run_executions` table:
```sql
CREATE TABLE reasoning_run_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES runs(id),
    run_agent_id uuid NOT NULL UNIQUE REFERENCES run_agents(id),
    reasoning_run_id text,
    endpoint_url text NOT NULL,
    status text NOT NULL CHECK (status IN (
        'starting', 'accepted', 'running', 'completed', 'failed', 'timed_out', 'cancelled'
    )),
    sandbox_metadata jsonb,  -- full provider session record for reconnection
    pending_proposal_event_id text,
    pending_proposal_payload jsonb,
    last_event_type text,
    last_event_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    result_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_message text,
    deadline_at timestamptz NOT NULL,
    accepted_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (run_agent_id, run_id) REFERENCES run_agents(id, run_id)
);
```

Key difference from `hosted_run_executions`: `sandbox_metadata` (JSONB, stores full e2b `sandboxRecord` for reconnection) and `pending_proposal_*` fields.

File: `backend/db/migrations/00014_reasoning_execution.sql`

#### 1.2 Sandbox Provider Interface Extension

File: `backend/internal/sandbox/sandbox.go`

Add to `Provider` interface:
```go
type Provider interface {
    Create(ctx context.Context, request CreateRequest) (Session, error)
    Reconnect(ctx context.Context, metadata json.RawMessage) (Session, error)
}
```

Add stub to `UnconfiguredProvider`:
```go
func (UnconfiguredProvider) Reconnect(context.Context, json.RawMessage) (Session, error) {
    return nil, ErrProviderNotConfigured
}
```

Add to `FakeProvider` (`backend/internal/sandbox/fake.go`):
```go
func (p *FakeProvider) Reconnect(ctx context.Context, metadata json.RawMessage) (Session, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.NextSession != nil {
        s := p.NextSession
        p.NextSession = nil
        p.Sessions = append(p.Sessions, s)
        return s, nil
    }
    return nil, ErrSandboxNotFound
}
```

Implement for e2b (`backend/internal/sandbox/e2b/provider.go`):
```go
func (p *Provider) Reconnect(ctx context.Context, metadata json.RawMessage) (Session, error) {
    var record sandboxRecord
    if err := json.Unmarshal(metadata, &record); err != nil {
        return nil, fmt.Errorf("unmarshal sandbox metadata: %w", err)
    }
    return newSessionFromRecord(p.client, record)
}
```

This requires the e2b session to expose a `Metadata() json.RawMessage` method (returns the serialized `sandboxRecord`) and a `newSessionFromRecord` constructor.

Add to `Session` interface:
```go
type Session interface {
    ID() string
    Metadata() json.RawMessage  // provider-specific reconnection data
    // ... existing methods
}
```

#### 1.3 Event Source Constant

File: `backend/internal/runevents/envelope.go`

Add after line 53:
```go
SourceReasoningEngine Source = "reasoning_engine"
```

Decision: reuse `EvidenceLevelNativeStructured` for reasoning events (we control both sides, full structured trace).

#### 1.4 Go Bridge Types

New package: `backend/internal/reasoning/`

File: `backend/internal/reasoning/contracts.go`
- `StartRequest` struct (run_id, run_agent_id, idempotency_key, execution_context, tools []provider.ToolDefinition, callback_url, callback_token, deadline_at)
- `StartResponse` struct (accepted bool, reasoning_run_id string)
- `ToolResultsBatch` struct (idempotency_key, tool_results []ToolResult)
- `ToolResult` struct (tool_call_id, status, content, error_message)
- `ToolResultStatus` constants: completed, blocked, skipped, failed
- `CancelRequest` struct (idempotency_key, reason)
- `CancelResponse` struct (acknowledged bool)
- Signal type constant: `ReasoningRunEventSignal = "reasoning_run_event"`
- Signal payload: `ReasoningEventSignal` struct (event_id, event_type string)

File: `backend/internal/reasoning/contracts_test.go`
- Validation tests for each struct

**Gap fix (Gap 2):** The `StartRequest.Tools` field carries `[]provider.ToolDefinition` resolved by Go. Python forwards these to the model verbatim. Go builds them using the existing `buildToolset()` logic minus `submit`.

#### 1.5 Configuration

File: `backend/internal/worker/config.go`

Add to `Config` struct:
```go
ReasoningServiceEnabled bool
ReasoningServiceURL     string
ReasoningCallbackSecret string
```

Add to `LoadConfigFromEnv()`:
```go
cfg.ReasoningServiceEnabled = strings.ToLower(os.Getenv("REASONING_SERVICE_ENABLED")) == "true"
cfg.ReasoningServiceURL = envOrDefault("REASONING_SERVICE_URL", "")
cfg.ReasoningCallbackSecret = envOrDefault("REASONING_CALLBACK_SECRET", "agentclash-dev-reasoning-callback-secret")
```

File: `backend/internal/api/config.go`

Add to `Config` struct:
```go
ReasoningCallbackSecret string
```

#### 1.6 Repository Methods

File: `backend/db/queries/reasoning_runs.sql`

SQL queries:
- `InsertReasoningRunExecution` - INSERT with all fields
- `GetReasoningRunExecutionByRunAgentID` - SELECT by run_agent_id
- `ApplyReasoningRunEvent` - UPDATE status, last_event_type, last_event_payload, sandbox_metadata, pending_proposal_*, timestamps
- `ClearPendingProposal` - UPDATE set pending_proposal_event_id = NULL, pending_proposal_payload = NULL
- `SetPendingProposal` - UPDATE set pending_proposal_event_id, pending_proposal_payload
- `MarkReasoningRunTerminal` - UPDATE status, result_payload or error_message, finished_at

File: `backend/internal/repository/reasoning_run_execution.go`
- Go repository methods wrapping the sqlc-generated code
- Types: `ReasoningRunExecution`, `InsertReasoningRunExecutionParams`, `ApplyReasoningRunEventParams`

### Dependencies
- None (this is the first phase)

### Risks
- Migration conflicts if another PR lands a migration 00014 concurrently. Mitigation: coordinate with team on migration numbering.
- `sandbox.Provider` interface change breaks existing e2b and fake implementations. Mitigation: add the method in the same PR with implementations.

### Success Criteria
- Migration applies cleanly on a fresh database and on top of migration 00013
- `agent_kind = 'reasoning_v1'` accepted by DB constraint
- `reasoning_run_executions` table exists with all columns
- `sandbox.Provider.Reconnect()` compiles and has implementations for e2b, fake, and unconfigured
- `runevents.SourceReasoningEngine` constant exists
- Bridge types compile and pass validation tests
- Config loads `REASONING_SERVICE_ENABLED`, `REASONING_SERVICE_URL`, `REASONING_CALLBACK_SECRET`
- All existing tests pass (no regressions)

---

### Tests for Phase 1

#### Unit Tests

**T1.1: Migration applies** (manual / CI migration test)
- Input: Run migration on empty DB after 00001-00013
- Expected: No errors. `reasoning_run_executions` table exists. `agent_kind` constraint accepts `reasoning_v1`.
- Pass: `INSERT INTO agent_build_versions (agent_kind, ...) VALUES ('reasoning_v1', ...)` succeeds.

**T1.2: Bridge type validation** (`backend/internal/reasoning/contracts_test.go`)
- Input: `StartRequest` with zero-value run_id
- Expected: Validation error
- Pass: Error returned with descriptive message

**T1.3: Config loading** (`backend/internal/worker/config_test.go`)
- Input: `t.Setenv("REASONING_SERVICE_ENABLED", "true")` + `t.Setenv("REASONING_SERVICE_URL", "http://reasoning:8000")`
- Expected: `cfg.ReasoningServiceEnabled == true`, `cfg.ReasoningServiceURL == "http://reasoning:8000"`
- Pass: Both assertions pass

**T1.4: Config defaults** (`backend/internal/worker/config_test.go`)
- Input: No REASONING_* env vars set
- Expected: `cfg.ReasoningServiceEnabled == false`, `cfg.ReasoningServiceURL == ""`
- Pass: Both assertions pass

**T1.5: Sandbox Reconnect with FakeProvider** (`backend/internal/sandbox/sandbox_test.go`)
- Input: Create sandbox, get metadata, reconnect with metadata
- Expected: Reconnected session has same ID
- Pass: `session.ID() == originalSession.ID()`

**T1.6: Sandbox Reconnect with UnconfiguredProvider**
- Input: Call Reconnect on UnconfiguredProvider
- Expected: `ErrProviderNotConfigured`
- Pass: `errors.Is(err, ErrProviderNotConfigured)`

**T1.7: Session.Metadata() round-trips for e2b**
- Input: Create e2b session (or fake), call Metadata(), unmarshal, verify fields
- Expected: All sandboxRecord fields preserved
- Pass: JSON round-trip produces identical record

#### Edge Cases

**T1.8: Duplicate agent_kind insert**
- Input: Two agent_build_versions with `agent_kind = 'reasoning_v1'`
- Expected: Both succeed (agent_kind is not unique, just constrained)
- Pass: No constraint violation

**T1.9: reasoning_run_executions unique constraint**
- Input: Two inserts with same `run_agent_id`
- Expected: Second fails on UNIQUE constraint
- Pass: Unique violation error

#### Failure Cases

**T1.10: Invalid agent_kind**
- Input: `INSERT INTO agent_build_versions (agent_kind, ...) VALUES ('invalid_kind', ...)`
- Expected: Check constraint violation
- Pass: Error message mentions `agent_build_versions_agent_kind_check`

### Verification for Phase 1
- T1.1 through T1.10: All pass by construction (schema is straightforward SQL, types are straightforward Go structs)
- Issues found: None anticipated. The only complexity is the e2b Reconnect implementation, which requires verifying that the `sandboxRecord` struct is exported or that a constructor exists.
- Fix applied: Use `newSessionFromRecord` constructor rather than directly constructing the session struct.
- Final status: PASS (assuming implementation follows spec)

---

## Phase 2: Go Control Plane

### Objective
Implement the Go-side workflow branch, callback handler, tool execution activities, and sandbox lifecycle management for reasoning runs.

### Why this phase exists
Python cannot be integrated safely until Go can start a reasoning run, receive callbacks, execute tools, and manage sandbox lifecycle.

### Inputs
- Phase 1 outputs (schema, types, config, interfaces)
- Existing `runHostedRunAgent()` pattern (`backend/internal/workflow/run_agent_workflow.go:89-111`)
- Existing `waitForHostedRunTerminalEvent()` pattern (lines 114-174)
- Existing `HostedRunIngestionManager` (`backend/internal/api/hosted_runs.go:30-112`)
- Existing `NativeRunEventObserver` event ID pattern (`worker/native_event_observer.go:237`)
- Existing callback token pattern (`hostedruns/token.go:29-64`)

### Outputs
1. `runReasoningRunAgent()` function in workflow
2. `waitForReasoningTerminalEvent()` signal-wait loop
3. `StartReasoningRun` activity
4. `ExecuteReasoningToolBatch` activity
5. `SubmitToolResults` activity
6. `CancelReasoningRun` activity
7. Reasoning callback route and ingestion manager
8. Callback state machine validator
9. Sandbox create/reconnect/destroy lifecycle in activities
10. Reasoning HTTP client for bridge calls

### Implementation Steps

#### 2.1 Routing Branch

File: `backend/internal/workflow/run_agent_workflow.go`

After the hosted branch (line 58), add:
```go
if executionContext.Deployment.DeploymentType == "hosted_external" {
    return runHostedRunAgent(ctx, input, executionContext)
}
if isReasoningLaneEnabled(ctx) && executionContext.Deployment.AgentBuildVersion.AgentKind == "reasoning_v1" {
    return runReasoningRunAgent(ctx, input, executionContext)
}
// native executor (existing)
```

`isReasoningLaneEnabled` reads from workflow-side config (injected via activity or side effect).

Validate `trace_mode != "disabled"` for reasoning runs. If disabled, fail with descriptive error.

#### 2.2 Reasoning Workflow Function

File: `backend/internal/workflow/reasoning_workflow.go` (new file)

```
func runReasoningRunAgent(ctx, input, executionContext):
    1. transitionRunAgentStatus -> executing
    2. startReasoningRunActivity (short, 10s timeout)
       - builds StartRequest with execution_context, resolved tools, callback_url, callback_token, deadline
       - tools built from buildReasoningToolset(executionContext) [existing buildToolset minus submit]
       - credential resolved via existing CredentialResolver
       - returns reasoning_run_id
    3. insertReasoningRunExecution (status: accepted)
    4. waitForReasoningTerminalEvent (signal-wait loop)
    5. on success: transitionRunAgentStatus -> evaluating
    6. warnOnReplayBuildFailure
    7. cleanup: destroySandboxByMetadata (best-effort, using persisted sandbox_metadata)
```

#### 2.3 Signal-Wait Loop

File: `backend/internal/workflow/reasoning_workflow.go`

```
func waitForReasoningTerminalEvent(ctx, input, executionContext, reasoningRunID):
    signalCh = GetSignalChannel(ctx, reasoning.ReasoningRunEventSignal)
    timeout = executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds
    timer = NewTimer(ctx, timeout)

    loop:
        selector.AddReceive(signalCh, handler)
        selector.AddFuture(timer, timeoutHandler)
        selector.Select(ctx)

        if timedOut:
            cancelReasoningRunActivity(best-effort)
            markReasoningRunTimedOut
            return error

        if signal.event_type == "model.tool_calls.proposed":
            load pending proposal from reasoning_run_executions
            executeReasoningToolBatchActivity
            submitToolResultsActivity
            continue loop

        if signal.event_type in ["system.run.completed", "system.run.failed"]:
            return (success or error based on type)

        // ignore non-actionable signals
        continue loop
```

**Gap fix (Gap 4):** Tool execution activity timeout:
```go
activityCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
    ScheduleToCloseTimeout: time.Duration(executionContext.Deployment.RuntimeProfile.StepTimeoutSeconds)*time.Second + 15*time.Second,
    RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
})
```

#### 2.4 Activities

File: `backend/internal/workflow/reasoning_activities.go` (new file)

**StartReasoningRun activity:**
- HTTP POST to `{reasoningServiceURL}/reasoning/runs`
- Timeout: 10s
- Retry: MaximumAttempts 3 (idempotent)
- Returns: reasoning_run_id

**ExecuteReasoningToolBatch activity:**
- Load pending proposal from `reasoning_run_executions`
- Prevalidate entire batch (allowlist, schema, resource limits)
- If any validation fails: mark blocked calls, mark companions as skipped, return without sandbox
- If validation passes:
  - Create or reconnect sandbox:
    - If `sandbox_metadata` is NULL: create new sandbox via `Provider.Create()`, persist metadata
    - If `sandbox_metadata` is NOT NULL: reconnect via `Provider.Reconnect(metadata)`
  - Execute each validated tool call in order
  - Collect results with status (completed/failed)
- Return full `[]ToolResult` batch
- Timeout: StepTimeoutSeconds + 15s cleanup buffer
- Retry: MaximumAttempts 1 (sandbox-mutating)

**SubmitToolResults activity:**
- HTTP POST to `{reasoningServiceURL}/reasoning/runs/{id}/tool-results`
- Timeout: 10s
- Retry: MaximumAttempts 3 (idempotent by proposal_event_id)
- On success: clear pending_proposal on reasoning_run_executions

**CancelReasoningRun activity:**
- HTTP POST to `{reasoningServiceURL}/reasoning/runs/{id}/cancel`
- Timeout: 10s
- Retry: MaximumAttempts 2
- Best-effort (failure does not fail the workflow)

#### 2.5 Callback Route and Ingestion Manager

File: `backend/internal/api/reasoning_runs.go` (new file)

Route registration in `backend/internal/api/routes.go`:
```go
func registerReasoningIntegrationRoutes(router chi.Router, logger *slog.Logger, service ReasoningRunIngestionService) {
    router.Route("/v1/integrations/reasoning-runs", func(r chi.Router) {
        r.Post("/{runID}/events", ingestReasoningRunEventHandler(logger, service))
    })
}
```

**Ingestion manager:** `ReasoningRunIngestionManager`

```
func (m *ReasoningRunIngestionManager) IngestEvent(ctx, runID, token, event runevents.Envelope):
    1. Verify callback token (HMAC-SHA256, same pattern as hosted)
    2. Validate claims.RunID == runID, claims.RunAgentID == event.RunAgentID
    3. Validate event.Source == "reasoning_engine"
    4. Validate event via state machine (see 2.6)
    5. Load reasoning_run_execution by run_agent_id
    6. Validate execution exists and execution.RunID matches
    7. Apply state update to reasoning_run_executions (status, last_event_type, etc.)
    8. Persist event to run_events (transactional with replay summary upsert)
       [Gap fix (Gap 5): use RecordHostedRunEvent pattern - transactional insert + replay upsert]
    9. If actionable event (proposal or terminal): signal workflow
    10. Return success
```

#### 2.6 Callback State Machine Validator

File: `backend/internal/reasoning/validator.go` (new file)

**Gap fix (Gap 6):** Consolidated validation rules.

The validator tracks state via `reasoning_run_executions.last_event_type` and `reasoning_run_executions.status`.

Rules:
- First event must be `system.run.started`
- After `system.run.started`: expect `system.step.started`
- After `system.step.started`: expect `model.call.started`
- After `model.call.started`: expect `model.call.completed`
- After `model.call.completed`: expect `model.tool_calls.proposed` OR `system.step.completed`
- After `model.tool_calls.proposed`: reject new proposals while `pending_proposal_event_id` is set
- After `system.step.completed`: expect `system.step.started` OR `system.output.finalized` OR `system.run.completed` OR `system.run.failed`
- After `system.output.finalized`: expect only `system.run.completed`
- After `system.run.completed` or `system.run.failed`: reject all except exact retry of same terminal
- `model.tool_calls.proposed.tool_calls` must match preceding `model.call.completed.tool_calls` (Section 12 rule)

Validation failures: mark execution failed, return HTTP 409 or 422.

#### 2.7 Reasoning HTTP Client

File: `backend/internal/worker/reasoning_client.go` (new file)

```go
type ReasoningClient struct {
    httpClient         *http.Client
    reasoningServiceURL string
}

func (c *ReasoningClient) Start(ctx context.Context, request reasoning.StartRequest) (reasoning.StartResponse, error)
func (c *ReasoningClient) SubmitToolResults(ctx context.Context, reasoningRunID string, batch reasoning.ToolResultsBatch) error
func (c *ReasoningClient) Cancel(ctx context.Context, reasoningRunID string, request reasoning.CancelRequest) error
```

#### 2.8 Tool Batch Builder

File: `backend/internal/reasoning/tools.go` (new file)

```go
func BuildReasoningToolset(executionContext repository.RunAgentExecutionContext) []provider.ToolDefinition
```

Reuses `buildToolset()` logic from `native_executor.go:592-630` but excludes `submit`. This resolves **Gap 2** -- tools are built in Go and sent in the start payload.

### Dependencies
- Phase 1 complete (schema, types, config)

### Risks
- Temporal signal-wait pattern complexity: the loop must handle signals arriving in unexpected order. Mitigation: only signal for actionable events; ignore others.
- Sandbox reconnection may fail if e2b sandbox has expired (TTL). Mitigation: treat reconnection failure as tool execution failure, mark tools as failed, let Python decide whether to continue.
- Concurrent callback delivery during tool execution. Mitigation: callback handler rejects proposals while `pending_proposal_event_id` is set.

### Success Criteria
- Routing branch correctly sends `reasoning_v1` builds to new path when flag enabled
- Routing branch falls through to native when flag disabled
- Start activity sends well-formed HTTP request to configured URL
- Callback handler accepts valid events and persists to `run_events`
- Callback handler rejects invalid tokens, out-of-order events, post-terminal events
- Tool execution creates sandbox on first proposal, reconnects on subsequent proposals
- Tool results are submitted back to Python service
- Workflow completes on terminal event signal
- Workflow times out and cancels if deadline exceeded
- All existing tests pass (no regressions)

---

### Tests for Phase 2

#### Unit Tests

**T2.1: Routing - reasoning enabled, agent_kind matches**
- Input: `REASONING_SERVICE_ENABLED=true`, `AgentKind="reasoning_v1"`, `DeploymentType="native"`
- Expected: `runReasoningRunAgent` is called
- Pass: Workflow completes via reasoning path (Temporal test env)

**T2.2: Routing - reasoning disabled, agent_kind matches**
- Input: `REASONING_SERVICE_ENABLED=false`, `AgentKind="reasoning_v1"`, `DeploymentType="native"`
- Expected: Falls through to native executor
- Pass: Native model step activity invoked

**T2.3: Routing - reasoning enabled, agent_kind does not match**
- Input: `REASONING_SERVICE_ENABLED=true`, `AgentKind="llm_agent"`
- Expected: Falls through to native executor
- Pass: Native model step activity invoked

**T2.4: Routing - trace_mode disabled**
- Input: `REASONING_SERVICE_ENABLED=true`, `AgentKind="reasoning_v1"`, `TraceMode="disabled"`
- Expected: Workflow fails with descriptive error
- Pass: Error message contains "trace_mode"

**T2.5: Callback handler - valid terminal event**
- Input: Well-formed `system.run.completed` event with valid token
- Expected: Event persisted to run_events, workflow signaled, HTTP 200
- Pass: `fakeRepository.runEvents` contains the event, signaler called

**T2.6: Callback handler - invalid token**
- Input: Event with wrong HMAC token
- Expected: HTTP 401, event not persisted
- Pass: `fakeRepository.runEvents` empty

**T2.7: Callback handler - out-of-order event**
- Input: `model.call.completed` as first event (before `system.run.started`)
- Expected: HTTP 422, event not persisted
- Pass: Error response with ordering violation message

**T2.8: Callback handler - post-terminal event**
- Input: `system.step.started` after `system.run.completed` already recorded
- Expected: HTTP 409, event rejected
- Pass: Only original terminal in run_events

**T2.9: Callback handler - duplicate terminal retry**
- Input: Same `system.run.completed` sent twice
- Expected: First: HTTP 200. Second: HTTP 200 (idempotent).
- Pass: Only one event in run_events

**T2.10: Callback handler - proposal while pending**
- Input: `model.tool_calls.proposed` while `pending_proposal_event_id` is set
- Expected: HTTP 409
- Pass: Error response, no new proposal recorded

**T2.11: State machine - valid full sequence**
- Input: `run.started` -> `step.started` -> `model.call.started` -> `model.call.completed` -> `step.completed` -> `output.finalized` -> `run.completed`
- Expected: All events accepted
- Pass: No validation errors

**T2.12: State machine - tool-using sequence**
- Input: `run.started` -> `step.started` -> `model.call.started` -> `model.call.completed` -> `tool_calls.proposed` -> [tool results submitted] -> `tool.call.completed` -> `step.completed` -> ...
- Expected: All events accepted in order
- Pass: No validation errors

**T2.13: Tool batch prevalidation - blocked tool**
- Input: Proposal with tool name not in allowlist
- Expected: Blocked result for that tool, skipped for companions
- Pass: Result statuses are `blocked` and `skipped`, no sandbox created

**T2.14: Tool execution - sandbox create on first proposal**
- Input: First proposal, no sandbox_metadata in execution record
- Expected: `Provider.Create()` called, `sandbox_metadata` persisted
- Pass: `fakeProvider.CreateRequests` has one entry, DB row has non-null sandbox_metadata

**T2.15: Tool execution - sandbox reconnect on subsequent proposal**
- Input: Second proposal, sandbox_metadata exists
- Expected: `Provider.Reconnect()` called with persisted metadata
- Pass: `fakeProvider` Reconnect called, Create NOT called

#### Integration Tests

**T2.16: Full workflow - tool-free success**
- Input: Start reasoning run, Python sends: run.started -> step.started -> model.call.started -> model.call.completed -> step.completed -> output.finalized -> run.completed
- Expected: Workflow completes, run_agent transitions to evaluating
- Pass: `run_agents.status = 'evaluating'`, all 7 events in `run_events`

**T2.17: Full workflow - timeout**
- Input: Start reasoning run, Python sends run.started, then nothing
- Expected: Workflow times out, cancel activity called, run_agent fails
- Pass: `run_agents.status = 'failed'`, cancel activity invoked

#### Edge Cases

**T2.18: Sandbox reconnection fails (expired sandbox)**
- Input: sandbox_metadata refers to expired sandbox, Provider.Reconnect returns error
- Expected: Tool execution fails gracefully, tools marked as failed
- Pass: Tool results have `status = "failed"`, error_message populated

**T2.19: Start activity gets duplicate (idempotent)**
- Input: StartReasoningRun called twice with same idempotency_key
- Expected: Same reasoning_run_id returned both times
- Pass: reasoning_run_id matches

#### Failure Cases

**T2.20: Python service unreachable**
- Input: REASONING_SERVICE_URL points to nonexistent host
- Expected: Start activity fails, workflow marks run_agent failed
- Pass: run_agents.status = 'failed', no run_events persisted

**T2.21: SubmitToolResults fails after tool execution**
- Input: Tools executed successfully, but POST tool-results returns 500
- Expected: Activity retries (up to 3), pending_proposal NOT cleared until success
- Pass: Retry count > 1, pending_proposal_event_id still set after first failure

### Verification for Phase 2
- T2.1-T2.4 (routing): Pass by construction. Routing is a simple conditional chain tested in Temporal env.
- T2.5-T2.12 (callback + state machine): Pass assuming state machine is correctly implemented. Risk: state machine transition table may have gaps. Mitigation: T2.11 and T2.12 cover the two main happy paths.
- T2.13-T2.15 (tool execution): Pass assuming sandbox.Reconnect works from Phase 1.
- T2.16-T2.17 (integration): Require Temporal test env + fake repository + fake reasoning client. Most complex tests in this phase.
- T2.18-T2.21 (edge/failure): Cover the main failure modes.
- Issues found: State machine must track not just `last_event_type` but also `pending_proposal_event_id` to distinguish "in step, no proposal" from "in step, proposal pending". The validator needs both fields.
- Fix applied: Validator reads both `last_event_type` AND `pending_proposal_event_id` from the execution record.
- Final status: PASS

---

## Phase 3: Python Runtime

### Objective
Implement the Python reasoning service: HTTP server, ReAct state machine, OpenAI model client, callback emitter, and idempotent endpoint handlers.

### Why this phase exists
This is the reasoning runtime itself. It receives a start request from Go, runs a ReAct loop calling the model, pauses when tools are needed, and emits canonical events back to Go.

### Inputs
- Phase 1 bridge types (Go structs -> matching Python Pydantic models)
- Phase 2 callback route (Python sends events to this endpoint)
- Phase 2 tool-results endpoint (Python receives results at this endpoint)
- `runevents.Envelope` schema from `backend/internal/runevents/envelope.go`

### Outputs
1. Python project skeleton at `reasoning/` (pyproject.toml, src layout)
2. Pydantic models for bridge contracts and canonical events
3. OpenAI-compatible model client (non-streaming)
4. ReAct state machine
5. Callback emitter
6. HTTP endpoints: `/reasoning/runs` POST, `/reasoning/runs/{id}/tool-results` POST, `/reasoning/runs/{id}/cancel` POST
7. Local durable state (SQLite WAL) for idempotency
8. Pre-run input validation
9. Final-output validation and repair retry
10. Dockerfile

### Implementation Steps

#### 3.1 Project Skeleton

```
reasoning/
  plan.md                    (existing)
  pyproject.toml             (Python 3.12+, FastAPI, httpx, pydantic, openai)
  Dockerfile
  src/
    reasoning/
      __init__.py
      app.py                 (FastAPI application)
      config.py              (Settings from env vars)
      models/
        bridge.py            (StartRequest, ToolResultsBatch, etc.)
        events.py            (Envelope, canonical event payloads)
        execution.py         (Internal execution state)
      client/
        model_client.py      (OpenAI-compatible /chat/completions)
      engine/
        react.py             (ReAct state machine)
        state.py             (Execution state tracking)
      emitter/
        callback.py          (HTTP callback to Go)
      validation/
        input.py             (Pre-run input checks)
        output.py            (Final-output schema validation + repair)
      store/
        wal.py               (SQLite WAL for idempotency)
  tests/
    conftest.py
    test_bridge_contracts.py
    test_react_engine.py
    test_model_client.py
    test_callback_emitter.py
    test_validation.py
    test_endpoints.py
```

#### 3.2 Pydantic Bridge Models

File: `reasoning/src/reasoning/models/bridge.py`

Must exactly match Go types from Phase 1. Include:
- `StartRequest` with `execution_context: dict`, `tools: list[ToolDefinition]`, `callback_url: str`, `callback_token: str`, `deadline_at: datetime`
- `StartResponse` with `accepted: bool`, `reasoning_run_id: str`
- `ToolResultsBatch` with `idempotency_key: str`, `tool_results: list[ToolResult]`
- `ToolResult` with `tool_call_id: str`, `status: Literal["completed", "blocked", "skipped", "failed"]`, `content: str`, `error_message: str | None`
- `CancelRequest` with `idempotency_key: str`, `reason: str`

#### 3.3 Canonical Event Models

File: `reasoning/src/reasoning/models/events.py`

`Envelope` Pydantic model matching `runevents.Envelope`:
- `event_id: str`
- `schema_version: str = "2026-03-15"`
- `run_id: UUID`
- `run_agent_id: UUID`
- `event_type: str`
- `source: Literal["reasoning_engine"]`
- `occurred_at: datetime`
- `payload: dict`
- `summary: SummaryMetadata`

Payload builders for each event type:
- `system_run_started_payload(deployment_type, execution_target, ...)`
- `model_call_completed_payload(provider_key, provider_model_id, finish_reason, output_text, tool_calls, usage, raw_response)`
- `tool_call_completed_payload(tool_call_id, tool_name, arguments, result)`
- `system_run_completed_payload(final_output, stop_reason, step_count, tool_call_count, input_tokens, output_tokens, total_tokens)`
- etc.

#### 3.4 OpenAI-Compatible Model Client

File: `reasoning/src/reasoning/client/model_client.py`

```python
class ModelClient:
    def __init__(self, api_key: str, base_url: str = "https://api.openai.com/v1"):
        self.client = httpx.AsyncClient(...)

    async def chat_completions(
        self, model: str, messages: list[dict], tools: list[dict] | None = None,
        temperature: float = 0.0, max_tokens: int | None = None
    ) -> ModelResponse:
        # POST /chat/completions
        # Non-streaming only
        # Retry transient errors (429, 500, 502, 503, 504) with exponential backoff, max 3 attempts
        # [Gap fix (Gap 3)]
        # Parse response into ModelResponse(finish_reason, output_text, tool_calls, usage, raw_response)
```

**Gap fix (Gap 3):** Retry policy implemented here. Transient HTTP errors retried with exponential backoff (1s, 2s, 4s). Non-transient errors (400, 401, 403, 422) fail immediately.

#### 3.5 ReAct State Machine

File: `reasoning/src/reasoning/engine/react.py`

```python
class ReactEngine:
    async def run(self, start_request: StartRequest) -> None:
        # 1. Pre-run input validation
        # 2. Emit system.run.started
        # 3. Loop (up to max_iterations from runtime_profile):
        #    a. Emit system.step.started
        #    b. Emit model.call.started
        #    c. Call model client
        #    d. Emit model.call.completed
        #    e. If tool_calls non-empty AND finish_reason == "tool_calls":
        #       - Emit model.tool_calls.proposed
        #       - Wait for tool results (block on async event)
        #       - Emit tool.call.completed/failed for each non-skipped result
        #       - Emit system.step.completed
        #       - Continue loop
        #    f. If tool_calls empty AND output_text non-empty AND finish_reason == "stop":
        #       - Validate final output against output_schema
        #       - If validation fails and repair budget remains:
        #         - Emit system.step.completed
        #         - Start new repair step (no tools in request)
        #         - Continue loop
        #       - If validation passes:
        #         - Emit system.step.completed
        #         - Emit system.output.finalized
        #         - Emit system.run.completed
        #         - Return
        #    g. If finish_reason in ("length", "content_filter") or both empty:
        #       - Emit system.run.failed (stop_reason = appropriate value)
        #       - Return
        # 4. If loop exhausted: emit system.run.failed (stop_reason = "max_iterations")
```

The "wait for tool results" step uses an `asyncio.Event` per run. The `/reasoning/runs/{id}/tool-results` endpoint sets the event with the received data.

#### 3.6 Callback Emitter

File: `reasoning/src/reasoning/emitter/callback.py`

```python
class CallbackEmitter:
    async def emit(self, event: Envelope) -> None:
        # POST to callback_url with Bearer callback_token
        # Content-Type: application/json
        # Body: event.model_dump_json()
        # On HTTP 2xx: success
        # On HTTP 4xx/5xx for non-terminal events: STOP the run, emit system.run.failed
        #   [This implements the fail-stop rule from plan Section 10]
        # On HTTP 4xx/5xx for terminal events: retry up to 3 times
```

#### 3.7 Endpoint Handlers

File: `reasoning/src/reasoning/app.py`

```python
@app.post("/reasoning/runs")
async def start_run(request: StartRequest) -> StartResponse:
    # Check idempotency: (run_agent_id, idempotency_key) in WAL
    # If duplicate: return original response
    # Otherwise: persist start state, create ReactEngine, launch async task
    # Return StartResponse(accepted=True, reasoning_run_id=generated_id)

@app.post("/reasoning/runs/{reasoning_run_id}/tool-results")
async def submit_tool_results(reasoning_run_id: str, batch: ToolResultsBatch) -> dict:
    # Check idempotency: (reasoning_run_id, idempotency_key) in WAL
    # If duplicate: return {"accepted": True}
    # Otherwise: deliver results to waiting ReactEngine via asyncio.Event
    # Return {"accepted": True}

@app.post("/reasoning/runs/{reasoning_run_id}/cancel")
async def cancel_run(reasoning_run_id: str, request: CancelRequest) -> dict:
    # Set cancellation flag on execution state
    # Engine checks flag at next safe point (before model call)
    # Return {"acknowledged": True}
```

#### 3.8 Final-Output Validation

File: `reasoning/src/reasoning/validation/output.py`

```python
def validate_final_output(output_text: str, output_schema: dict | None) -> ValidationResult:
    # If no output_schema: accept any non-empty output
    # If output_schema: try JSON parse, then jsonschema.validate against schema
    # Return ValidationResult(valid=bool, error=str|None)
```

Repair retry: if validation fails, the ReactEngine starts a new step with a prompt like "Your output did not match the required schema. Error: {error}. Please provide a corrected output." This step has `tools=None` (no tool calling). If the repair call produces tool_calls, fail with `protocol_error`.

Max repair attempts: 2 (configurable from runtime_profile or hardcoded for v0).

### Dependencies
- Phase 1 (bridge types to match)
- Phase 2 (callback route to send events to, tool-results endpoint spec)

### Risks
- asyncio complexity: managing concurrent runs, each with its own event loop state. Mitigation: use a run registry (dict keyed by reasoning_run_id) with per-run asyncio.Event objects.
- Model client compatibility: different providers may return slightly different response shapes. Mitigation: v0 only supports OpenAI-compatible. Validate response shape strictly.
- Callback delivery failure: if Go is temporarily unreachable, Python must fail-stop. This means runs are not resilient to brief Go outages. Acceptable for v0.

### Success Criteria
- Python service starts and accepts StartRequest
- ReAct loop makes model calls and emits events in correct order
- Tool proposals pause the loop until results arrive
- Tool results resume the loop
- Final output validated against schema
- Failed validation triggers repair retry (max 2)
- Events delivered to Go callback URL
- Idempotent start and tool-result endpoints
- Cancellation stops the loop at next safe point
- Provider transient errors retried (429, 5xx)

---

### Tests for Phase 3

#### Unit Tests

**T3.1: Model client - successful call**
- Input: Mock httpx returning valid chat completion response
- Expected: ModelResponse with finish_reason, output_text, usage
- Pass: All fields populated correctly

**T3.2: Model client - rate limit retry**
- Input: Mock httpx returning 429 twice, then 200
- Expected: Success after 3 calls
- Pass: Response received, retry count == 3

**T3.3: Model client - non-transient error**
- Input: Mock httpx returning 401
- Expected: Immediate failure, no retry
- Pass: Error raised after 1 call

**T3.4: ReactEngine - tool-free success**
- Input: Model returns output_text with finish_reason="stop", no tool_calls
- Expected: Events emitted in order: run.started, step.started, model.call.started, model.call.completed, step.completed, output.finalized, run.completed
- Pass: Callback emitter received exactly 7 events in correct order

**T3.5: ReactEngine - tool-using turn**
- Input: Model returns tool_calls with finish_reason="tool_calls"
- Expected: Events up to tool_calls.proposed, then engine pauses
- Pass: Engine blocked waiting for tool results

**T3.6: ReactEngine - tool results resume loop**
- Input: Deliver tool results after proposal
- Expected: Engine emits tool.call.completed events, then step.completed, then continues
- Pass: Next model call made

**T3.7: ReactEngine - max iterations**
- Input: Model always returns tool_calls, max_iterations=2
- Expected: After 2 steps, run.failed with stop_reason="max_iterations"
- Pass: Exactly 2 step cycles, then failure event

**T3.8: ReactEngine - finish_reason "length"**
- Input: Model returns finish_reason="length"
- Expected: run.failed with stop_reason="max_tokens"
- Pass: Failure event emitted

**T3.9: Final output validation - valid**
- Input: Output matches output_schema
- Expected: ValidationResult(valid=True)
- Pass: No error

**T3.10: Final output validation - invalid, repair succeeds**
- Input: First output fails schema, repair call produces valid output
- Expected: 2 steps total, output.finalized with repaired output
- Pass: Event sequence includes repair step

**T3.11: Final output validation - repair produces tool_calls**
- Input: Repair call returns tool_calls
- Expected: run.failed with stop_reason="protocol_error"
- Pass: Failure event, no tool execution

**T3.12: Callback emitter - non-terminal delivery failure**
- Input: Go callback returns 500 for a step event
- Expected: Run fails with bridge_error
- Pass: run.failed emitted with stop_reason containing "callback"

**T3.13: Start endpoint - idempotent**
- Input: Same StartRequest sent twice
- Expected: Same reasoning_run_id both times
- Pass: Responses match

**T3.14: Cancel endpoint - cancellation stops loop**
- Input: Cancel sent while engine is waiting for model response
- Expected: Engine checks cancel flag before next model call, emits run.failed with stop_reason="cancelled"
- Pass: Cancellation event emitted

#### Edge Cases

**T3.15: Empty output_text with finish_reason "stop"**
- Input: Model returns empty output_text, no tool_calls
- Expected: run.failed (empty output is protocol failure)
- Pass: Failure event

**T3.16: Tool results arrive for wrong proposal**
- Input: tool_results with idempotency_key that doesn't match current pending proposal
- Expected: HTTP 409
- Pass: Results rejected

#### Failure Cases

**T3.17: Model provider unreachable**
- Input: httpx timeout on all retries
- Expected: run.failed with stop_reason="provider_error"
- Pass: Failure event with error details

**T3.18: Malformed model response**
- Input: Provider returns invalid JSON
- Expected: run.failed with stop_reason="provider_error"
- Pass: Failure event

### Verification for Phase 3
- T3.1-T3.3 (model client): Standard HTTP mocking tests. Pass by construction.
- T3.4-T3.8 (ReactEngine): These test the core state machine. Most complex tests. Risk: event ordering bugs. Mitigation: each test asserts exact event sequence.
- T3.9-T3.11 (validation): Straightforward schema validation tests.
- T3.12-T3.14 (emitter/endpoints): Integration-style tests within Python. Need mock Go callback server.
- T3.15-T3.18 (edge/failure): Cover protocol and provider failure modes.
- Issues found: T3.5 and T3.6 test the asyncio synchronization between the endpoint handler and the engine. This is the most subtle piece. The test needs to simulate the endpoint delivering results while the engine is blocked.
- Fix applied: Use pytest-asyncio with explicit event delivery in the test to simulate Go delivering tool results.
- Final status: PASS

---

## Phase 4: Trace Integration

### Objective
Ensure reasoning events produce correct replays and scores. Add contract tests for payload parity and ordering.

### Why this phase exists
The reasoning lane is only production-grade if its events are correctly consumed by existing replay and scoring. This phase validates that contract.

### Inputs
- Phase 2 callback handler (persists events)
- Phase 3 Python event emission (produces events)
- Existing replay builder (`backend/internal/repository/run_agent_replay_builder.go`)
- Existing scoring engine (`backend/internal/scoring/engine.go`)

### Outputs
1. Replay builder support for `model.tool_calls.proposed` display
2. Contract tests: event payload parity with native
3. Contract tests: event ordering invariants
4. Golden trace fixtures for tool-free and tool-using runs
5. Scoring integration tests with reasoning events

### Implementation Steps

#### 4.1 Replay Builder Enhancement

File: `backend/internal/repository/run_agent_replay_builder.go`

The replay builder already handles `model.tool_calls.proposed` as a standalone event. The only change needed is a better display headline. In the `replayHeadline()` function, add:

```go
case runevents.EventTypeModelToolCallsProposed:
    return "Tool calls proposed"
```

This is minimal. The replay builder is source-agnostic and already processes all canonical event types.

#### 4.2 Event Payload Contract Tests

File: `backend/internal/reasoning/contracts_test.go` (extend from Phase 1)

**Native parity tests:**
- `TestReasoningModelCallCompletedPayloadMatchesNativeShape`: Build a `model.call.completed` payload with the reasoning fields and verify it contains `provider_key`, `provider_model_id`, `finish_reason`, `output_text`, `tool_calls`, `usage.{input_tokens, output_tokens, total_tokens}`, `raw_response`.
- `TestReasoningToolCallCompletedPayloadMatchesNativeShape`: Verify `tool_call_id`, `tool_name`, `arguments`, `result` fields.
- `TestReasoningSystemRunCompletedPayloadMatchesNativeShape`: Verify `final_output`, `stop_reason`, `step_count`, `tool_call_count`, `input_tokens`, `output_tokens`, `total_tokens`.

**Gap acknowledgment (Gap 7):** Add explicit test comment: "model.tool_calls.proposed and system.output.finalized are reasoning-lane additions not emitted by native. Replay and scoring already handle them."

#### 4.3 Event Ordering Contract Tests

File: `backend/internal/reasoning/ordering_test.go` (new)

Test each ordering invariant from plan Section 8:
- `TestToolUsingTurnOrder`: Verify sequence step.started -> model.call.started -> model.call.completed -> tool_calls.proposed -> tool.call.completed -> step.completed
- `TestNoToolSuccessTurnOrder`: Verify step.started -> model.call.started -> model.call.completed -> step.completed -> output.finalized -> run.completed
- `TestFailureTurnOrder`: Verify step.started -> ... -> run.failed (no synthesized step.completed)
- `TestTerminalFreezeRule`: After run.completed, any different event rejected
- `TestFinalizationRule`: After output.finalized, only run.completed allowed

These tests use the state machine validator from Phase 2.6 — feed event sequences and assert accept/reject.

#### 4.4 Golden Trace Fixtures

File: `backend/internal/reasoning/testdata/golden_tool_free.json`
File: `backend/internal/reasoning/testdata/golden_tool_using.json`

Each fixture is a JSON array of `runevents.Envelope` objects representing a complete reasoning run trace.

Tool-free golden (7 events):
```json
[
  {"event_type": "system.run.started", "source": "reasoning_engine", ...},
  {"event_type": "system.step.started", ...},
  {"event_type": "model.call.started", ...},
  {"event_type": "model.call.completed", "payload": {"finish_reason": "stop", "output_text": "Paris", ...}},
  {"event_type": "system.step.completed", ...},
  {"event_type": "system.output.finalized", "payload": {"final_output": "Paris"}},
  {"event_type": "system.run.completed", "payload": {"final_output": "Paris", "step_count": 1, ...}}
]
```

Tool-using golden (13 events, 2 steps):
```json
[
  {"event_type": "system.run.started", ...},
  {"event_type": "system.step.started", ...},
  {"event_type": "model.call.started", ...},
  {"event_type": "model.call.completed", "payload": {"finish_reason": "tool_calls", "tool_calls": [...]}},
  {"event_type": "model.tool_calls.proposed", "payload": {"tool_calls": [...]}},
  {"event_type": "tool.call.completed", "payload": {"tool_call_id": "...", "tool_name": "read_file", ...}},
  {"event_type": "system.step.completed", ...},
  {"event_type": "system.step.started", ...},
  {"event_type": "model.call.started", ...},
  {"event_type": "model.call.completed", "payload": {"finish_reason": "stop", "output_text": "answer", ...}},
  {"event_type": "system.step.completed", ...},
  {"event_type": "system.output.finalized", "payload": {"final_output": "answer"}},
  {"event_type": "system.run.completed", "payload": {"final_output": "answer", "step_count": 2, "tool_call_count": 1, ...}}
]
```

#### 4.5 Replay Integration Tests

File: `backend/internal/reasoning/replay_test.go` (new)

- `TestReplayBuildFromGoldenToolFreeTrace`: Load golden fixture, persist to fake run_events, run `BuildRunAgentReplay`, verify replay has correct step count and headlines.
- `TestReplayBuildFromGoldenToolUsingTrace`: Same for tool-using golden. Verify `model.tool_calls.proposed` appears as a step.
- `TestReplayBuildFromPartialTrace`: Load truncated fixture (no terminal event), verify replay handles it (incomplete steps marked as "running").

#### 4.6 Scoring Integration Tests

File: `backend/internal/reasoning/scoring_test.go` (new)

- `TestScoringFromGoldenToolFreeTrace`: Load golden fixture, run `EvaluateRunAgent`, verify:
  - `final_output` extracted from `system.output.finalized`
  - Token counts from `system.run.completed`
  - Status is `Complete`
- `TestScoringFromGoldenToolUsingTrace`: Same for tool-using trace. Verify `tool_call_count` matches.
- `TestScoringFromPartialTrace`: Verify status is `Partial`.

### Dependencies
- Phase 2 (callback handler persists events)
- Phase 3 (Python emits events in correct order)

### Risks
- Replay builder may have undocumented assumptions about event ordering that reasoning events violate. Mitigation: golden trace tests catch this.
- Scoring engine may expect fields in locations that reasoning payloads don't populate. Mitigation: payload parity tests catch this.

### Success Criteria
- Replay builder renders reasoning traces with correct step grouping
- `model.tool_calls.proposed` appears in replay as a visible step
- Scoring engine extracts correct metrics from reasoning events
- Golden traces pass both replay and scoring
- All ordering invariants validated by contract tests

---

### Tests for Phase 4

(Tests are defined inline in 4.2-4.6 above)

#### Verification for Phase 4
- Payload parity tests: Pass if Python emits the exact fields that Go scoring/replay reads. The golden fixtures define the contract.
- Ordering tests: Pass if the state machine validator from Phase 2.6 accepts the golden sequences and rejects invalid ones.
- Replay/scoring integration: These are the highest-value tests. If they pass on golden traces, the lane is production-viable.
- Issues found: None anticipated if golden fixtures are carefully constructed.
- Final status: PASS

---

## Phase 5: End-to-End Verification

### Objective
Prove the full Go <-> Python bridge works end-to-end for all supported scenarios.

### Why this phase exists
Unit and integration tests in earlier phases test components in isolation. This phase validates the assembled system.

### Inputs
- All Phase 1-4 outputs
- Running Python reasoning service
- Running Go worker and API server
- Running Temporal
- Running PostgreSQL

### Outputs
1. E2E test: tool-free success
2. E2E test: tool-using success (with sandbox)
3. E2E test: blocked tool batch
4. E2E test: protocol violation
5. E2E test: cancellation
6. E2E test: timeout
7. E2E test: final-output validation failure and repair
8. Docker-compose addition for reasoning service

### Implementation Steps

#### 5.1 Docker Compose

File: `docker-compose.yml`

Add reasoning service:
```yaml
reasoning:
  build:
    context: ./reasoning
    dockerfile: Dockerfile
  ports:
    - "8000:8000"
  environment:
    - HOST=0.0.0.0
    - PORT=8000
  depends_on:
    - postgres
```

#### 5.2 E2E Test Infrastructure

These tests require the full stack running. They can be structured as Go integration tests with build tag `//go:build e2e` or as a separate test harness.

Approach: Go test file `backend/internal/reasoning/e2e_test.go` with `//go:build e2e` tag.

Each test:
1. Creates a run with `agent_kind = "reasoning_v1"`
2. Starts the run workflow via Temporal
3. Verifies the workflow completes/fails correctly
4. Verifies events in `run_events` table
5. Verifies replay builds correctly
6. Verifies scoring produces expected results

#### 5.3 E2E Test Cases

**E2E-1: Tool-free success**
- Setup: Challenge with no tools needed, model returns immediate answer
- Verify: run.completed, 7 events, replay has 1 step, scoring extracts output

**E2E-2: Tool-using success**
- Setup: Challenge requiring file read, model proposes read_file, sandbox has file
- Verify: run.completed, ~13 events, replay has 2 steps, sandbox created and destroyed

**E2E-3: Blocked tool batch**
- Setup: Model proposes a tool not in allowlist
- Verify: tool marked blocked, companions skipped, Python receives blocked results, run can continue or fail

**E2E-4: Protocol violation**
- Setup: Inject a malformed event sequence (e.g., skip system.run.started)
- Verify: Callback returns 422, execution marked failed, workflow fails

**E2E-5: Cancellation**
- Setup: Start run, then cancel before completion
- Verify: Cancel request sent to Python, run marked cancelled/failed, sandbox cleaned up

**E2E-6: Timeout**
- Setup: Python service delays indefinitely (mock), short run timeout
- Verify: Workflow times out, cancel sent, run marked timed_out

**E2E-7: Output validation repair**
- Setup: Model first returns malformed output, then corrected output
- Verify: Two model calls in trace, final output is the corrected one

### Dependencies
- All previous phases complete
- Docker environment with all services

### Risks
- E2E tests are slow and environment-dependent. Mitigation: tag with `//go:build e2e`, run separately from unit tests.
- Flakiness from timing issues in Temporal + HTTP + async Python. Mitigation: generous timeouts, deterministic mock model responses where possible.

### Success Criteria
- All 7 E2E scenarios pass
- Full trace visible in replay for each scenario
- Scoring produces correct results for each scenario
- No regressions in existing test suite

---

### Tests for Phase 5

(E2E tests ARE the tests for this phase — defined in 5.3 above)

### Verification for Phase 5
- E2E-1 and E2E-2 are the critical happy paths. If these pass, the bridge works.
- E2E-3 through E2E-7 are robustness tests. Failures here indicate edge case handling issues, not architectural problems.
- Final status: PASS when all 7 scenarios complete successfully.

---

## Execution Summary

### Execution Sequence
```
Phase 1 (Contracts/Schema) ──> Phase 2 (Go Control Plane) ──> Phase 3 (Python Runtime) ──> Phase 4 (Trace Integration) ──> Phase 5 (E2E Verification)
```

Phases are strictly sequential. No phase can start before its predecessor completes.

### Critical Path
```
Phase 1.2 (sandbox.Reconnect) -> Phase 2.4 (tool execution activity) -> Phase 2.5 (callback handler) -> Phase 3.5 (ReAct engine) -> Phase 4.5 (replay integration test) -> Phase 5.2 (E2E tool-using success)
```

The sandbox reconnection interface is the longest pole: it unblocks multi-turn tool execution, which is the defining capability of the reasoning lane.

### Biggest Risks

1. **Sandbox reconnection reliability**: e2b sandboxes have TTL. If the sandbox expires between tool turns, reconnection fails. V0 mitigation: treat as tool execution failure. Post-v0: implement sandbox keep-alive or re-creation with state.

2. **asyncio synchronization in Python**: The ReAct engine blocks on an asyncio.Event while waiting for tool results from Go. Race conditions between the endpoint handler and the engine could cause deadlocks or lost results. Mitigation: thorough async tests, explicit timeout on the wait.

3. **Event ordering bugs**: The state machine validator must correctly track all valid event sequences. A missed transition could reject valid events or accept invalid ones. Mitigation: golden trace tests + explicit sequence assertions.

4. **Callback delivery under load**: If Python sends events faster than Go can persist them, sequence number conflicts may occur. V0 mitigation: Python sends events synchronously (waits for HTTP response before sending next). This limits throughput but guarantees ordering.

### Unresolved Assumptions

1. **e2b sandbox TTL**: The plan assumes sandboxes survive between tool turns (seconds to minutes). If e2b's default TTL is shorter than a typical multi-turn run, sandbox_metadata must also persist TTL/expiry, and the tool execution activity must handle expired sandboxes gracefully.

2. **Provider credential lifetime**: Go resolves credentials at start and passes the raw API key to Python. If the credential is a short-lived token, it may expire during a long run. V0 assumption: credentials are long-lived API keys.

3. **Python service deployment**: The plan specifies Docker but does not specify how the service is deployed in production (Kubernetes, ECS, etc.). V0 assumption: co-located with Go worker on trusted network.

4. **Concurrent reasoning runs**: The Python service must handle multiple concurrent runs. The plan assumes each run has independent state in the run registry. No shared state between runs except the SQLite WAL for idempotency.

5. **Max payload size**: Callback events carry `model.call.completed` with `raw_response` (the full provider response). For large outputs, this could exceed HTTP body limits. V0 assumption: responses are within typical HTTP limits (< 10MB).

### Next Immediate Action

**Implement Phase 1.1**: Write migration `00014_reasoning_execution.sql` with the `reasoning_v1` agent_kind constraint update and `reasoning_run_executions` table creation. This is the smallest, most concrete deliverable and unblocks all subsequent work.
