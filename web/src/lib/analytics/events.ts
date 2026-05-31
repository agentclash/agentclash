/**
 * Typed event taxonomy for PostHog captures fired from the web app.
 *
 * Keep names dot-namespaced (`web.<surface>.<action>`) so they group cleanly
 * in PostHog. The backend emits its own `cli.command.invoked` / `api.request`
 * events from the Go middleware, and the worker emits `run.*` — do not
 * duplicate those here.
 *
 * When you add a new event:
 *   1. Add it to WEB_EVENTS below.
 *   2. Add the typed properties to WebEventPayloads.
 *   3. Update the event taxonomy in `docs/analytics.md`.
 */

export const WEB_EVENTS = {
  /** User completed WorkOS login and the session hydrated. */
  AUTH_LOGIN_SUCCESS: "web.auth.login.success",
  /** User created a new organization (first-time onboarding). */
  ORG_CREATED: "web.org.created",
  /** User created a new workspace. */
  WORKSPACE_CREATED: "web.workspace.created",
  /** User added a provider API key (OpenAI, Anthropic, etc). */
  PROVIDER_ACCOUNT_ADDED: "web.provider_account.added",
  /** User created or uploaded a challenge pack. */
  PACK_UPLOADED: "web.pack.uploaded",
  /** User created a run through the web UI. */
  RUN_CREATED: "web.run.created",
  /** User promoted a run failure to a regression case. */
  REGRESSION_CASE_PROMOTED: "web.regression.case_promoted",
} as const;

export type WebEventName = (typeof WEB_EVENTS)[keyof typeof WEB_EVENTS];

export interface WebEventPayloads {
  [WEB_EVENTS.AUTH_LOGIN_SUCCESS]: { user_id: string };
  [WEB_EVENTS.ORG_CREATED]: { organization_id: string };
  [WEB_EVENTS.WORKSPACE_CREATED]: { workspace_id: string; organization_id: string };
  [WEB_EVENTS.PROVIDER_ACCOUNT_ADDED]: { workspace_id: string; provider: string };
  [WEB_EVENTS.PACK_UPLOADED]: { workspace_id: string; pack_id?: string };
  [WEB_EVENTS.RUN_CREATED]: { workspace_id: string; run_id?: string };
  [WEB_EVENTS.REGRESSION_CASE_PROMOTED]: { workspace_id: string; case_id?: string };
}
