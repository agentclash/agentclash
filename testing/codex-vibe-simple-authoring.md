# Vibe simple authoring regression contract

## Functional behavior

- New conversational drafts use plain-English examples and success criteria. Go constructs the nonempty-output check, single semantic evaluator, dimension references and case keys, then uses the merged pack compiler. The model does not author regexes or internal scoring configuration.
- Preserve every supplied example and the entire success criterion. Invalid or oversized drafts fail explicitly with the existing single repair, never by dropping coverage. Explicit imports and accepted evaluation contracts remain unchanged.
- Proposed requirements are strings with server-owned provenance; model output cannot assert accepted status or user authorship. Assumptions are visibly labeled proposals. Existing accepted requirements survive unchanged.
- A sparse task description gets one useful question when necessary. Once the task is known, or the user delegates defaults, provide a useful draft with stated assumptions. Unknown product claims, prices, discounts and measured benefits stay unspecified. An existing agent without supplied instructions is described as an unconnected sample, never as the user's tested agent.
- Costs, call/context caps, model roles and provider retry policy do not change.
- Preserve safe provider error categories (rate limit, authentication, timeout, unavailable, invalid request) instead of collapsing them into execution_error. Do not expose provider-supplied bodies/messages, infer zero cost, or retry uncertain calls. Accept both numeric OpenRouter and string OpenAI error-code payloads without losing the HTTP classification.
- For an operator-verified endpoint supporting structured outputs, send a strict JSON Schema for authoring. Allow at most three proposed requirements and two assumptions (five total); the model can combine related clauses without dropping evaluation coverage. Count the complete schema in the initial and repair context. Keep local validation authoritative because provider enforcement varies. Unsupported/unverified profiles retain bounded JSON-object authoring; there is no automatic provider downgrade or retry after schema rejection.

## Unit tests

- Compile a plain-English marketing draft with persuasive copy and a CTA into a valid merged pack; retain all examples and criteria, keep adversarial test strings exactly, and bind the selected evaluator.
- Reject empty/oversized examples, invalid requirement shapes and unknown internal model-generated configuration. No silently accepted regex or discarded case.
- Keep existing explicit import, role independence, immutable evaluation and repair-context tests passing.
- Verify the schema reaches the selected provider request, array limits agree with local validation, schema overhead is counted during correction, and invalid server-claimed schema output still fails the local boundary.
- Unit-test numeric and string provider error codes and safe category mapping, including wrapped errors and arbitrary secret-bearing provider text that must never appear in the Vibe fault.

## Integration / functional tests

- Real isolated PostgreSQL with a fake provider: first-response success, one bounded correction, two invalid responses, and null draft after correction. Persist only validated output and server-attributed proposed requirements.
- An accepted agent improvement cannot change its evaluation even if the model supplies different examples.

## Smoke and E2E tests

- Build API and worker, run affected Go race tests and vet, restart only idle local services.
- On the actual localhost conversation, submit a new continuation of the user's marketing-copy brief through the normal bounded free-model path. Inspect the reply, assumptions and generated evaluation; preserve prior failed attempts. Do not reset any trial or retry uncertain calls.
- Check the draft through the UI if the existing trial permits it; otherwise report the limit and verify execution with isolated test fixtures. No paid model, provider fallback, or public deployment.

## Manual review

- Reproduce: “I have an agent that needs testing” → “generates content” → “marketing copy” / “use the best info you have” → “persuasive. CTA imp”. A useful draft should be reachable without invented quantitative product claims or repeated intake questions.
- Live model quality remains probabilistic. Record the actual outcome and any failure; successful mocked tests are not proof of live model reliability.
