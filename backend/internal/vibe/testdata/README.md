# Founder conversation regression inputs

`founder_context.json` contains sanitized operation inputs from the four recorded September 2026 simulations: research idea, existing research agent, receptionist idea, and existing voice agent. They are simulated conversations, not customer interviews.

Identifiers are replaced deterministically. Administrative timestamps and actor metadata are removed. The conversation text, requirements and evaluation contracts remain intact so the tests exercise the context failures seen in the audit, rather than shortened examples. No credentials, browser state, raw provider journals or private endpoints are included.

`TestVibeFounderContext` verifies the complete initial and bounded repair requests against the supported free profile and reconstructs the compact blueprint to compare every field with the original. The onboarding integration fixtures separately verify journey routing and authoring boundaries with deterministic responses. Neither fixture claims live model quality; that requires the capped live verification described in the delivery report.
