/**
 * Typed payload shapes for run event envelopes delivered over SSE.
 *
 * Mirrors the payload maps produced by the backend observers in
 * `backend/internal/worker/native_event_observer.go`,
 * `prompt_eval_event_observer.go`, and `backend/internal/runevents/hosted.go`.
 *
 * We keep these loose (`Partial`/optional fields) because:
 *   1. Hosted-engine payloads don't share the exact native schema.
 *   2. New event types can be added before the UI is updated.
 *
 * For arena rendering we only care about a small, resilient subset.
 */

export type RunEventType =
  | "system.run.started"
  | "system.run.completed"
  | "system.run.failed"
  | "system.output.finalized"
  | "system.step.started"
  | "system.step.completed"
  | "model.call.started"
  | "model.call.completed"
  | "model.output.delta"
  | "model.tool_calls.proposed"
  | "tool.call.started"
  | "tool.call.completed"
  | "tool.call.failed"
  | "sandbox.command.started"
  | "sandbox.command.completed"
  | "sandbox.command.failed"
  | "sandbox.file.read"
  | "sandbox.file.written"
  | "sandbox.file.listed"
  | "grader.verification.file_captured"
  | "grader.verification.directory_listed"
  | "grader.verification.code_executed"
  | "scoring.started"
  | "scoring.metric.recorded"
  | "scoring.completed"
  | "scoring.failed";

export interface ModelCallStartedPayload {
  provider_key?: string;
  model?: string;
  provider_model_id?: string;
  message_count?: number;
  tool_definition_count?: number;
}

export interface ModelCallCompletedPayload {
  provider_key?: string;
  provider_model_id?: string;
  finish_reason?: string;
  output_text?: string;
  usage?: {
    input_tokens?: number;
    output_tokens?: number;
    total_tokens?: number;
  };
}

export interface ModelOutputDeltaPayload {
  provider_key?: string;
  provider_model_id?: string;
  stream_kind?: "text" | "tool_call" | string;
  text_delta?: string;
  tool_call_fragment?: {
    index?: number;
    id_fragment?: string;
    name_fragment?: string;
    arguments_fragment?: string;
  };
}

export interface ToolCallPayload {
  tool_call_id?: string;
  tool_name?: string;
  tool_category?: string;
  resolved_tool_name?: string;
  arguments?: unknown;
  result?: {
    content?: string;
    is_error?: boolean;
  };
}

export interface SandboxCommandPayload {
  command?: string;
  args?: string[];
  cwd?: string;
  exit_code?: number;
  duration_ms?: number;
  stdout?: string;
  stderr?: string;
}

export interface SandboxFilePayload {
  path?: string;
  bytes?: number;
}

export interface StepPayload {
  step_index?: number;
}

export interface ScoringMetricPayload {
  metric_key?: string;
  score?: number;
  passed?: boolean;
}

export interface RunCompletedPayload {
  final_output?: string;
  step_count?: number;
  tool_call_count?: number;
  total_tokens?: number;
}

export interface RunFailedPayload {
  error?: string;
  stop_reason?: string;
  step_index?: number;
}
