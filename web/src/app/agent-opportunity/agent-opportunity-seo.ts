export const AGENT_OPPORTUNITY_PATH = "/agent-opportunity";

/** Primary queries: build vs buy, ROI/business case, should-we-build intent. */
export const AGENT_OPPORTUNITY_TITLE =
  "AI Agent ROI Calculator & Build vs Buy Assessment | AgentClash";

export const AGENT_OPPORTUNITY_DESCRIPTION =
  "Free AI agent opportunity report from any company URL. Get agentic AI use cases, conservative ROI ranges, build vs buy guidance, risk scoring, and an AgentClash evaluation plan before you ship.";

export const AGENT_OPPORTUNITY_KEYWORDS = [
  "AI agent ROI calculator",
  "AI agent ROI assessment",
  "should we build an AI agent",
  "build vs buy AI agent",
  "agentic AI use cases",
  "AI agent business case",
  "AI agent opportunity assessment",
  "AI agent evaluation",
  "agentic AI customer support",
  "AI agent pilot",
  "AI agent risk assessment",
  "when to build AI agents",
  "enterprise AI agents",
  "agent evaluation framework",
];

export const AGENT_OPPORTUNITY_H1 =
  "Should your company build an AI agent?";

export const AGENT_OPPORTUNITY_LEDE =
  "Paste a company URL for a free AI agent ROI and build vs buy assessment. We research the web, score agentic AI use cases, surface risks, and generate an evaluation plan you can run in AgentClash.";

export const AGENT_OPPORTUNITY_FAQ = [
  {
    question: "Should my company build an AI agent?",
    answer:
      "Only if you have a repeatable workflow with clear success criteria, acceptable risk, and enough volume to justify automation. Many teams should evaluate first or run a narrow pilot instead of shipping a customer-facing agent. This scanner returns an honest build, pilot, evaluate-first, or not-yet verdict from public evidence.",
  },
  {
    question: "How do you calculate AI agent ROI?",
    answer:
      "The report uses conservative ranges, not fake precision. It estimates monthly hours saved and cost impact from workflows visible on your site plus web research, then weighs that against implementation complexity and failure risk. Treat the output as a business-case starting point, not a CFO-ready forecast.",
  },
  {
    question: "What is the build vs buy decision for AI agents?",
    answer:
      "Build when the workflow is core IP, highly proprietary, or too niche for vendors. Buy or partner when the workflow is common, speed matters, and vendor boundaries are acceptable. Most enterprises land on a hybrid model. The report calls out which path fits the workflows it finds on your site.",
  },
  {
    question: "What are common agentic AI use cases?",
    answer:
      "High-volume support triage, onboarding guidance, sales qualification, developer docs assistance, and internal ops workflows are the most common starting points. Strong candidates have structured inputs, tool access, and measurable outcomes. Weak candidates are open-ended chat on your homepage with no escalation path.",
  },
  {
    question: "When should we run an AI agent evaluation before launch?",
    answer:
      "Before any customer-facing agent touches policy, billing, refunds, healthcare, or financial advice. AgentClash recommends realistic and adversarial eval cases tied to the workflow you plan to ship. This report includes a starter eval pack you can turn into an eval in AgentClash.",
  },
  {
    question: "How is this different from a chatbot ROI calculator?",
    answer:
      "Chatbot calculators assume a bot belongs on your site. This tool starts with whether an agent is worth building at all, then scores agentic workflows, risk, and eval readiness. It is built for teams deciding between pilot, buy, build, or wait.",
  },
] as const;

export const AGENT_OPPORTUNITY_SECTIONS = [
  {
    title: "AI agent ROI without the hype",
    body: "Most teams are pressured to ship agentic AI before they can defend the business case. This free AI agent ROI assessment turns a public company URL into workflow fit, savings ranges, and a risk profile you can share with product, support, and finance stakeholders.",
  },
  {
    title: "Build vs buy for enterprise AI agents",
    body: "The build vs buy AI agent decision depends on workflow uniqueness, data sensitivity, and how fast you need learning. The scanner highlights where a narrow pilot, vendor agent, or custom build is the safer path, and where you should not build yet.",
  },
  {
    title: "Agent evaluation before production",
    body: "An AI agent evaluation should happen before customers see the workflow. AgentClash turns the recommended cases from this report into same-task agent evals with replay evidence, so you can compare models and catch failure modes early.",
  },
] as const;
