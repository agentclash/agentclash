# codex/xai-provider-adapter — Test Contract

## Functional Behavior
- The worker provider router accepts `provider_key: "xai"` and routes requests through an xAI client.
- The xAI client uses xAI's OpenAI-compatible REST surface with bearer auth and the `https://api.x.ai/v1` base URL by default.
- Existing OpenAI-compatible request behavior still works for xAI text responses, streaming responses, and function/tool calls.
- xAI provider failures are normalized through the same failure model used by the other adapters.
- Judge model inference recognizes Grok model names and resolves a default `env://XAI_API_KEY` credential when no matching provider account is attached.
- Workspace users can create an xAI provider account from the provider-account UI picker.
- Existing provider behavior for OpenAI, Anthropic, Gemini, OpenRouter, and Mistral does not regress.

## Unit Tests
- `TestRouterRoutesToConcreteAdapter` or equivalent wiring coverage includes `xai` in the provider router map.
- Provider conformance coverage includes xAI for:
  - Simple text streaming response
  - Single tool call response
  - Auth error normalization
  - Rate-limit normalization
  - Empty or malformed stream handling
- Dedicated xAI provider tests verify:
  - Default base URL is `https://api.x.ai/v1`
  - Requests send bearer auth and `/chat/completions`
  - Streaming/tool-call parsing matches the current xAI contract
- Judge tests cover:
  - Grok model name infers provider key `xai`
  - Default credential reference resolves to `env://XAI_API_KEY`
- Web/UI tests cover the xAI provider option in the provider-account creation flow if there is existing coverage for that component.

## Integration / Functional Tests
- Worker bootstrap constructs a provider router with the new xAI client and does not break existing provider registration.
- A prompt-eval smoke path can be configured for xAI with `XAI_API_KEY` and a Grok model name.

## Smoke Tests
- `go test ./backend/internal/provider ./backend/internal/workflow ./backend/cmd/worker/...`
- If credentials are available, run xAI-specific provider smoke coverage and confirm a non-empty streamed response plus timing metadata.

## E2E Tests
- N/A — not applicable for this change beyond existing workspace/provider setup flows.

## Manual / cURL Tests
```bash
curl https://api.x.ai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $XAI_API_KEY" \
  -d '{
    "model": "grok-4.20-reasoning",
    "messages": [
      {"role": "user", "content": "Reply with exactly: smoke-ok"}
    ],
    "stream": false
  }'
# Expected: 200 with a chat completion payload from xAI.
```

```bash
go test ./backend/internal/provider -run 'TestConformance|TestXAI'
# Expected: xAI adapter tests pass with the existing provider suite.
```
