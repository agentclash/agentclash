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

  // Public agent-tryouts funnel (anonymous visitors trying an agent).
  /** Anonymous tryout session successfully launched. */
  TRYOUT_SESSION_STARTED: "web.tryout.session_started",
  /** Tryout launch failed (quota, rate limit, or other error). */
  TRYOUT_LAUNCH_FAILED: "web.tryout.launch_failed",
  /** User sent a follow-up message in a tryout session. */
  TRYOUT_MESSAGE_SENT: "web.tryout.message_sent",
  /** User ended a tryout session. */
  TRYOUT_SESSION_ENDED: "web.tryout.session_ended",
  /** User clicked a sign-in / save CTA from the tryout surface. */
  TRYOUT_SIGNUP_CTA_CLICKED: "web.tryout.signup_cta_clicked",
  /** User clicked the "talk to us" CTA in the tryout ROI calculator. */
  TRYOUT_ROI_CTA_CLICKED: "web.tryout.roi_cta_clicked",

  // Adjacent lead-capture surfaces.
  /** Visitor submitted a marketing resource lead form. */
  RESOURCE_LEAD_SUBMITTED: "web.resource.lead_submitted",
  /** Agent-opportunity report generated for a visitor. */
  AGENT_OPPORTUNITY_REPORT_GENERATED: "web.agent_opportunity.report_generated",
  /** Visitor clicked an offer in the top-of-page marketing promo banner. */
  PROMO_BANNER_CLICKED: "web.marketing.promo_banner_clicked",
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
  [WEB_EVENTS.TRYOUT_SESSION_STARTED]: {
    tryout_id: string;
    template_slug: string;
    model_key: string;
  };
  [WEB_EVENTS.TRYOUT_LAUNCH_FAILED]: { template_slug: string; status_code?: number };
  [WEB_EVENTS.TRYOUT_MESSAGE_SENT]: { tryout_id: string; message_length: number };
  [WEB_EVENTS.TRYOUT_SESSION_ENDED]: { tryout_id: string };
  [WEB_EVENTS.TRYOUT_SIGNUP_CTA_CLICKED]: { location: string; tryout_id?: string };
  [WEB_EVENTS.TRYOUT_ROI_CTA_CLICKED]: { template_slug?: string; email_domain?: string };
  [WEB_EVENTS.RESOURCE_LEAD_SUBMITTED]: {
    source: string;
    resource: string;
    intent?: string;
    email_domain?: string;
  };
  [WEB_EVENTS.AGENT_OPPORTUNITY_REPORT_GENERATED]: {
    verdict: string;
    use_case_count: number;
    company_size?: string;
    current_pain?: string;
  };
  [WEB_EVENTS.PROMO_BANNER_CLICKED]: {
    /** Which offer was clicked, e.g. "agent_opportunity" or "tryout". */
    offer: string;
    /** Destination route the offer links to. */
    destination: string;
    /** Page surface the banner rendered on, e.g. "home", "blog", "benchmarks". */
    page: string;
  };
}
