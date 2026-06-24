// Homepage FAQ. Rendered visibly in the landing page AND emitted as FAQPage
// JSON-LD on the homepage (see app/page.tsx) — kept in one place so the
// structured data always mirrors the visible content (a Google requirement).
// Answers are self-contained and answer-shaped so answer engines can extract
// them directly.
export const HOME_FAQ: Array<{ question: string; answer: string }> = [
  {
    question: "What is AgentClash?",
    answer:
      "AgentClash is an open-source AI agent evaluation platform. It runs your agents on real tasks with the same tools and constraints, captures replayable failure evidence, scores the full trajectory, and lets you promote failed runs into permanent regression tests.",
  },
  {
    question:
      "How is AgentClash different from prompt-evaluation tools like LangSmith or Braintrust?",
    answer:
      "Prompt-evaluation tools score the text a model returns from a single call. AgentClash evaluates multi-turn agents that take actions in a real sandbox and scores the whole trajectory — tool choices, cost, latency, and recovery — not just the final answer. See agentclash.dev/compare for a side-by-side.",
  },
  {
    question: "Can I run AgentClash in CI?",
    answer:
      "Yes. AgentClash compares a candidate run against a baseline and fails CI when the candidate regresses on the scorecard or release gate you define. Failed runs can be promoted into permanent regression tests that replay in every future eval.",
  },
  {
    question: "Is AgentClash open source, and can I self-host it?",
    answer:
      "Yes. AgentClash is open source under the MIT license. You can self-host the full stack or run against the hosted backend, and the CLI installs from npm as the agentclash package.",
  },
  {
    question: "Which models and providers does AgentClash support?",
    answer:
      "300+ models via OpenRouter, plus first-class OpenAI, Anthropic, Gemini, xAI, Mistral, and OpenRouter providers. Tool calls are normalised to a single schema across providers so evals stay comparable.",
  },
  {
    question: "How do I get started with AgentClash?",
    answer:
      "Install the CLI with npm install -g agentclash (or run against the hosted backend), then follow the quickstart to author a eval pack and run your first agent eval.",
  },
];
