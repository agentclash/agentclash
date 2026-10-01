# Free-model local pilot contract

User authorized only free OpenRouter inference and localhost startup.

- Free-only mode rejects paid models and missing/ambiguous pricing. Explicit approved free profiles have zero token rates, exact model/provider routing, no fallback, and zero prompt/completion/request price ceilings.
- Preserve all existing context, output, graph, authorization, journal, Stop, concurrency and trial limits. Zero-price reservations and attempts remain durable; unknown accounting remains unknown. At most 40 free provider attempts per UTC day across this installation.
- API defaults, persisted session defaults and frontend defaults use the configured free model. Paid mode defaults stay unchanged.
- Real smoke check uses synthetic content and a free endpoint. No top-up, paid fallback or production deployment. Credential remains ignored and mode 0600.
- Tests: reject paid/zero-unknown/route-mismatched profiles; zero-cost admission/journaling/settlement; daily cap; paid regression suite; browser defaults and a live local chat/check.
- Live verification follow-up: newly created sessions serialize an empty operation array; conversational pack instructions explain that all checks apply to every case and use conditional semantic assertions rather than contradictory global phrase checks. Preserve earlier results and accepted criteria; do not rewrite them to improve a score.
