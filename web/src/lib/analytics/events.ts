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
  /** A successful WorkOS callback was joined to an internal user. */
  AUTH_COMPLETED: "web.auth.completed",
  /** One event per identified PostHog session. */
  APP_SESSION_STARTED: "web.app.session_started",
  /** A conversion CTA on a marketing or tryout surface was clicked. */
  MARKETING_CTA_CLICKED: "web.marketing.cta.clicked",
  /** A low-cardinality product setup step became visible. */
  SETUP_STEP_VIEWED: "web.setup.step.viewed",
  /** A user acted on a low-cardinality product setup step. */
  SETUP_STEP_CLICKED: "web.setup.step.clicked",
  /** User promoted a run failure to a regression case. */
  REGRESSION_CASE_PROMOTED: "web.regression.case_promoted",

  VIBE_BUILD_ENTRY_SELECTED: "web.vibe.build.entry_selected",
  VIBE_BUILD_MESSAGE_ADMITTED: "web.vibe.build.message_admitted",
  VIBE_BUILD_CLARIFICATION_VIEWED: "web.vibe.build.clarification_viewed",
  VIBE_BUILD_DEMO_SELECTED: "web.vibe.build.demo_selected",
  VIBE_BUILD_RESULT_VIEWED: "web.vibe.build.result_viewed",
  VIBE_BUILD_EXAMPLE_VIEWED: "web.vibe.build.example_viewed",
  VIBE_BUILD_ACTION_CLICKED: "web.vibe.build.action_clicked",
  VIBE_BUILD_TRIAL_REPLY_VIEWED: "web.vibe.build.trial_reply_viewed",
  VIBE_BUILD_RECOVERY_CLICKED: "web.vibe.build.recovery_clicked",
  VIBE_BUILD_EXPORT_REQUESTED: "web.vibe.build.export_requested",

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

export type VibeBuildEventBase = {
  schema_version: 1;
  door: "build";
  session_id: string;
  viewport: "mobile" | "tablet" | "desktop";
  artifact_id?: string;
  operation_id?: string;
  sample?: boolean;
};

export interface WebEventPayloads {
  [WEB_EVENTS.VIBE_BUILD_ENTRY_SELECTED]: VibeBuildEventBase & { entry_source: "two_door" | "return" };
  [WEB_EVENTS.VIBE_BUILD_MESSAGE_ADMITTED]: VibeBuildEventBase & { recipient: "guide" | "prototype" };
  [WEB_EVENTS.VIBE_BUILD_CLARIFICATION_VIEWED]: VibeBuildEventBase & { question_id: string; clarification_count: number };
  [WEB_EVENTS.VIBE_BUILD_DEMO_SELECTED]: VibeBuildEventBase & { sample_id: "email" };
  [WEB_EVENTS.VIBE_BUILD_RESULT_VIEWED]: VibeBuildEventBase & { outcome: "all_pass" | "failure" | "unassessed" | "incomplete"; passed: number; failed: number; unknown: number };
  [WEB_EVENTS.VIBE_BUILD_EXAMPLE_VIEWED]: VibeBuildEventBase & { case_ordinal: number; elapsed_ms?: number };
  [WEB_EVENTS.VIBE_BUILD_ACTION_CLICKED]: VibeBuildEventBase & { action: "try" | "checks" | "rules" | "change" | "tougher" | "save" | "export" | "review_problem" | "review_fix" };
  [WEB_EVENTS.VIBE_BUILD_TRIAL_REPLY_VIEWED]: VibeBuildEventBase & { thread_id?: string };
  [WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED]: VibeBuildEventBase & { action: "stop" | "retry" | "alternate_assistant" | "reload_evidence"; error_code?: string };
  [WEB_EVENTS.VIBE_BUILD_EXPORT_REQUESTED]: VibeBuildEventBase & { format: "json" };
  [WEB_EVENTS.AUTH_COMPLETED]: { provider: "workos" };
  [WEB_EVENTS.APP_SESSION_STARTED]: { posthog_session_id: string };
  [WEB_EVENTS.MARKETING_CTA_CLICKED]: {
    cta_id: string;
    intent: string;
    placement: string;
    source_path: string;
    destination_kind: string;
    destination_path?: string;
  };
  [WEB_EVENTS.SETUP_STEP_VIEWED]: {
    step: string;
    surface: "onboarding_wizard" | "activation_banner" | "activation_checklist";
    workspace_id?: string;
  };
  [WEB_EVENTS.SETUP_STEP_CLICKED]: {
    step: string;
    action: string;
    surface: "onboarding_wizard" | "activation_banner" | "activation_checklist";
    workspace_id?: string;
  };
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
