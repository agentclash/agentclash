import type { SeoPageConfig } from "./types";

type EditorialData = Pick<SeoPageConfig, "editorial" | "proofPoints" | "workflow" | "relatedLinks">;

export const EDITORIAL: Record<string, EditorialData> = {
  "/open-source-ai-agent-evaluation": {
    editorial: { title: "What can I inspect and operate myself?", example: "Inspect expr-eval-arena.yaml and the self-host setup.", limitation: "Self-hosting requires operating the stack" },
    proofPoints: ["Read the expression evaluator YAML and its scoring configuration.", "Follow the self-host setup before running your own workload."],
    workflow: [{ title: "Inspect the pack", text: "Read the expression evaluator YAML and its scoring configuration." }, { title: "Operate the stack", text: "Follow the self-host setup before running your own workload." }],
    relatedLinks: [
      { title: "Self host", text: "Set up and operate the AgentClash stack.", href: "/docs/getting-started/self-host" },
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
    ],
  },
  "/agent-evals": {
    editorial: { title: "How do I run my first eval?", example: "Define a small YAML task with a check, validate and publish the pack, then create a run with a deployment.", limitation: "A single task is a starting point, not coverage of production" },
    proofPoints: ["Define a task, tools and evaluation checks in YAML.", "Validate and publish the pack, then create a run using a deployment."],
    workflow: [{ title: "Write a small pack", text: "Define a task, tools and evaluation checks in YAML." }, { title: "Validate and run", text: "Validate and publish the pack, then create a run using a deployment." }],
    relatedLinks: [
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
      { title: "CI/CD agent gates", text: "Turn a passing eval into a release gate.", href: "/docs/guides/ci-cd-agent-gates" },
    ],
  },
  "/llm-agent-evaluation": {
    editorial: { title: "Why inspect the trajectory as well as the answer?", example: "An evaluator can return -3 for -7/2 yet still fail if its final output misses the required contract. Inspect the trajectory and final output together.", limitation: "A replay alone does not prove safety or correctness" },
    proofPoints: ["Review the evaluator solution and final output contract.", "Compare tool actions with the final outcome to understand how it was reached."],
    workflow: [{ title: "Check the answer", text: "Review the evaluator solution and final output contract." }, { title: "Inspect the sequence", text: "Compare tool actions with the final outcome to understand how it was reached." }],
    relatedLinks: [
      { title: "How AgentClash scores agent trajectories", text: "Understand trajectory scoring and its dimensions.", href: "/blog/how-agentclash-scores-agent-trajectories" },
      { title: "GPT generations expression evaluator", text: "Read the reported expression evaluator comparison and caveats.", href: "/benchmarks/gpt-generations-expression-evaluator" },
    ],
  },
  "/agent-evaluation-framework": {
    editorial: { title: "What components should I compare?", example: "Compare a candidate using a defined workload, allowed tools, scoring checks and a selected baseline. Check each component before choosing a framework.", limitation: "A checklist does not establish parity with every other tool" },
    proofPoints: ["Check workload definitions, execution policy and scoring.", "Compare baseline selection and gate configuration in the CI guide."],
    workflow: [{ title: "Compare components", text: "Check workload definitions, execution policy and scoring." }, { title: "Review release controls", text: "Compare baseline selection and gate configuration in the CI guide." }],
    relatedLinks: [
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
      { title: "CI/CD agent gates", text: "Configure and interpret a CI gate.", href: "/docs/guides/ci-cd-agent-gates" },
    ],
  },
  "/ai-agent-testing": {
    editorial: { title: "How do deterministic checks, judges and regression cases fit together?", example: "Check that -7/2 returns -3 with a deterministic test, use a judge for explanation quality, and retain a failure as a regression case.", limitation: "Judges vary and tests only cover asserted behavior" },
    proofPoints: ["Use deterministic hidden tests for expression behavior and a judge for configured qualitative dimensions.", "Turn observed failures into regression cases and rerun them after changes."],
    workflow: [{ title: "Assert behavior", text: "Use deterministic hidden tests for expression behavior and a judge for configured qualitative dimensions." }, { title: "Retain failures", text: "Turn observed failures into regression cases and rerun them after changes." }],
    relatedLinks: [
      { title: "How AgentClash scores agent trajectories", text: "Understand trajectory scoring and its dimensions.", href: "/blog/how-agentclash-scores-agent-trajectories" },
      { title: "CI/CD agent gates", text: "Configure and interpret a CI gate.", href: "/docs/guides/ci-cd-agent-gates" },
    ],
  },
  "/agent-trajectory-evaluation": {
    editorial: { title: "What can the sequence reveal?", example: "Compare the reported solution outcome with the final output contract; inspect tool calls in your own run trajectory.", limitation: "Traces provide evidence, not automatic proof that each action was appropriate" },
    proofPoints: ["Inspect tool calls and observations in a run trajectory.", "Check the final output contract alongside the solution."],
    workflow: [{ title: "Read the sequence", text: "Inspect tool calls and observations in a run trajectory." }, { title: "Compare the outcome", text: "Check the final output contract alongside the solution." }],
    relatedLinks: [
      { title: "Agent replay", text: "Explore run trajectory review.", href: "/features/agent-replay" },
      { title: "GPT generations expression evaluator", text: "Read the reported expression evaluator comparison and caveats.", href: "/benchmarks/gpt-generations-expression-evaluator" },
    ],
  },
  "/ci-cd-agent-evaluation": {
    editorial: { title: "How do I configure a gate and interpret failure?", example: "Select a workload, candidate, baseline and policy in the manifest. If the gate fails, inspect its output and run replay before deciding what to change.", limitation: "A stale or unsuitable baseline can make a gate failure misleading; thresholds and workload coverage also limit what the gate detects" },
    proofPoints: ["Pin a workload, candidate, baseline and policy in the gate configuration.", "Read gate output and review the run before deciding whether to refresh the baseline."],
    workflow: [{ title: "Configure a manifest", text: "Pin a workload, candidate, baseline and policy in the gate configuration." }, { title: "Investigate failure", text: "Read gate output and review the run before deciding whether to refresh the baseline." }],
    relatedLinks: [
      { title: "CI/CD agent gates", text: "Configure and interpret a CI gate.", href: "/docs/guides/ci-cd-agent-gates" },
      { title: "Dataset CI gates", text: "Set up gates for dataset workloads.", href: "/docs/guides/dataset-ci-gates" },
    ],
  },
  "/ai-agent-benchmark": {
    editorial: { title: "How should I compare agents on one workload?", example: "Expression evaluator uses identical shell/filesystem, no network and ten iterations.", limitation: "One task and one run per model cannot rank models generally" },
    proofPoints: ["Use the same task, shell and filesystem access, network policy and iteration budget for each candidate.", "Compare the reported score dimensions with the one-run sample limitation."],
    workflow: [{ title: "Hold conditions constant", text: "Use the same task, shell and filesystem access, network policy and iteration budget for each candidate." }, { title: "Read the report", text: "Compare the reported score dimensions with the one-run sample limitation." }],
    relatedLinks: [
      { title: "GPT generations expression evaluator", text: "Read the reported expression evaluator comparison and caveats.", href: "/benchmarks/gpt-generations-expression-evaluator" },
      { title: "Methodology", text: "Learn how to design and interpret a comparison.", href: "/benchmarks/methodology" },
    ],
  },
  "/agent-reliability-benchmark": {
    editorial: { title: "How can repetitions expose inconsistent outcomes?", example: "Repeat a pinned task and count passes, rather than infer reliability from one outcome.", limitation: "The public expression report is n=1 per model and cannot estimate a pass rate" },
    proofPoints: ["Fix the workload and evaluation configuration before repetitions.", "Run multiple repetitions and report passes and sample size rather than relying on one result."],
    workflow: [{ title: "Pin a task", text: "Fix the workload and evaluation configuration before repetitions." }, { title: "Count outcomes", text: "Run multiple repetitions and report passes and sample size rather than relying on one result." }],
    relatedLinks: [
      { title: "Pass@k reliability for enterprise teams", text: "Explore repeated trials and reliability.", href: "/blog/pass-k-reliability-enterprise-teams" },
      { title: "Methodology", text: "Learn how to design and interpret a comparison.", href: "/benchmarks/methodology" },
    ],
  },
  "/use-cases/coding-agent-evaluation": {
    editorial: { title: "What should a coding fixture check?", example: "Expression evaluator tests precedence, unary minus and truncating division.", limitation: "Its Python exercise is only one kind of coding work" },
    proofPoints: ["Specify the integer expression evaluator and its input contract.", "Test precedence, unary minus and truncation toward zero for division."],
    workflow: [{ title: "Define the fixture", text: "Specify the integer expression evaluator and its input contract." }, { title: "Check edge cases", text: "Test precedence, unary minus and truncation toward zero for division." }],
    relatedLinks: [
      { title: "GPT generations expression evaluator", text: "Read the reported expression evaluator comparison and caveats.", href: "/benchmarks/gpt-generations-expression-evaluator" },
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
    ],
  },
  "/use-cases/research-agent-evaluation": {
    editorial: { title: "How might I check research work?", example: "Inspect deep-research-analyst.yaml for its task and evaluation configuration.", limitation: "Fixture performance does not establish factual accuracy on unseen sources" },
    proofPoints: ["Read the deep research analyst task and evaluation configuration.", "Check the fixture's criteria and verify claims against sources appropriate to the task."],
    workflow: [{ title: "Inspect the research pack", text: "Read the deep research analyst task and evaluation configuration." }, { title: "Review the output", text: "Check the fixture's criteria and verify claims against sources appropriate to the task." }],
    relatedLinks: [
      { title: "Deep research analyst pack", text: "Inspect the research task and evaluation configuration in the example pack.", href: "https://github.com/agentclash/agentclash/blob/main/examples/challenge-packs/deep-research-analyst.yaml" },
      { title: "Evaluation spec reference", text: "Review available evaluation configuration.", href: "/docs/challenge-packs/evaluation-spec-reference" },
    ],
  },
  "/use-cases/support-agent-evaluation": {
    editorial: { title: "How can a support conversation be exercised?", example: "The multi-turn-refund-recovery.yaml pack provides a multi-turn refund scenario.", limitation: "A simulated refund does not measure real customer outcomes" },
    proofPoints: ["Use the multi-turn refund recovery scenario to define the exchange.", "Inspect how the agent handles the simulated refund and its configured evaluation."],
    workflow: [{ title: "Model a conversation", text: "Use the multi-turn refund recovery scenario to define the exchange." }, { title: "Check the response", text: "Inspect how the agent handles the simulated refund and its configured evaluation." }],
    relatedLinks: [
      { title: "Multi-turn refund recovery pack", text: "Inspect the multi-turn refund scenario and its evaluation setup.", href: "https://github.com/agentclash/agentclash/blob/main/examples/challenge-packs/multi-turn-refund-recovery.yaml" },
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
    ],
  },
  "/industries/banking": {
    editorial: { title: "How might a bank test a financial assistant?", example: "Hypothetical policy-bound refund or sensitive-data scenario adapted from multi-turn-refund-recovery.yaml and secret-hygiene-env.yaml.", limitation: "This scenario is illustrative; it does not evidence a customer deployment, regulatory approval or compliance result." },
    proofPoints: ["Adapt a refund or secret-hygiene example to a hypothetical financial assistant.", "Define explicit checks for sensitive data and the expected response."],
    workflow: [{ title: "Design a scenario", text: "Adapt a refund or secret-hygiene example to a hypothetical financial assistant." }, { title: "Check policy boundaries", text: "Define explicit checks for sensitive data and the expected response." }],
    relatedLinks: [
      { title: "Multi-turn refund recovery pack", text: "Inspect the refund scenario adapted by the banking example.", href: "https://github.com/agentclash/agentclash/blob/main/examples/challenge-packs/multi-turn-refund-recovery.yaml" },
      { title: "Secret hygiene pack", text: "Inspect the sensitive-data scenario adapted by the banking example.", href: "https://github.com/agentclash/agentclash/blob/main/examples/challenge-packs/secret-hygiene-env.yaml" },
      { title: "Security evaluation", text: "Design security evaluation scenarios.", href: "/docs/guides/security-evaluation" },
    ],
  },
  "/industries/insurance": {
    editorial: { title: "How might an insurer test a claims assistant?", example: "Hypothetical claim triage with a required artifact, using pack authoring as the fixture pattern.", limitation: "No insurance-specific pack or compliance result is claimed" },
    proofPoints: ["Describe a hypothetical claim triage task and a required output artifact.", "Encode expected fields and failure conditions in a challenge pack."],
    workflow: [{ title: "Define a claims fixture", text: "Describe a hypothetical claim triage task and a required output artifact." }, { title: "Specify checks", text: "Encode expected fields and failure conditions in a challenge pack." }],
    relatedLinks: [
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
    ],
  },
  "/industries/government": {
    editorial: { title: "How might a public-service team test an assistant?", example: "Hypothetical service-request fixture with bounded tools and a checked response.", limitation: "No government deployment, certification or legal compliance is implied" },
    proofPoints: ["Define a hypothetical service request with a limited tool policy.", "Specify the expected response and review the sandbox configuration."],
    workflow: [{ title: "Bound the tools", text: "Define a hypothetical service request with a limited tool policy." }, { title: "Check the response", text: "Specify the expected response and review the sandbox configuration." }],
    relatedLinks: [
      { title: "Sandbox and E2B", text: "Review sandbox configuration and limits.", href: "/docs/challenge-packs/sandbox-and-e2b" },
    ],
  },
  "/glossary/agent-evaluation": {
    editorial: { title: "What is an agent evaluation?", example: "A task with an agent run and checks, such as the expression evaluator.", limitation: "Its scope depends on task and scoring choices" },
    proofPoints: ["An evaluation starts with a task and an agent run.", "Specify how the expression evaluator's output will be assessed."],
    workflow: [{ title: "Define a task", text: "An evaluation starts with a task and an agent run." }, { title: "Add checks", text: "Specify how the expression evaluator's output will be assessed." }],
    relatedLinks: [
      { title: "Agent evals", text: "Start with an agent evaluation.", href: "/agent-evals" },
      { title: "Evaluation spec reference", text: "Review available evaluation configuration.", href: "/docs/challenge-packs/evaluation-spec-reference" },
    ],
  },
  "/glossary/challenge-pack": {
    editorial: { title: "What is a challenge pack?", example: "Inspect the expression evaluator YAML workload and scoring setup.", limitation: "A pack is a defined scenario, not a guarantee of general performance" },
    proofPoints: ["Inspect the expression evaluator task, tools and scoring setup.", "Use the pack as a defined workload and inspect its evaluation evidence."],
    workflow: [{ title: "Read the YAML", text: "Inspect the expression evaluator task, tools and scoring setup." }, { title: "Run the scenario", text: "Use the pack as a defined workload and inspect its evaluation evidence." }],
    relatedLinks: [
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
    ],
  },
  "/glossary/release-gate": {
    editorial: { title: "What is a release gate?", example: "Compare a candidate against a selected baseline and configured threshold in the CI manifest.", limitation: "Gate quality depends on chosen workload and thresholds" },
    proofPoints: ["Choose a reference run and workload in the CI manifest.", "Compare the candidate against the configured gate and inspect failures."],
    workflow: [{ title: "Select a baseline", text: "Choose a reference run and workload in the CI manifest." }, { title: "Apply thresholds", text: "Compare the candidate against the configured gate and inspect failures." }],
    relatedLinks: [
      { title: "CI/CD agent gates", text: "Configure and interpret a CI gate.", href: "/docs/guides/ci-cd-agent-gates" },
    ],
  },
  "/features/agent-scorecards": {
    editorial: { title: "What goes into and comes out of a scorecard?", example: "Expression evaluator shows correctness, reliability and normalized token-efficiency scores.", limitation: "Dimensions and aggregation are workload-specific" },
    proofPoints: ["The expression evaluator produces a scored run under its evaluation configuration.", "Compare correctness, reliability and normalized token efficiency with the report's caveats."],
    workflow: [{ title: "Provide a run", text: "The expression evaluator produces a scored run under its evaluation configuration." }, { title: "Read the scorecard", text: "Compare correctness, reliability and normalized token efficiency with the report's caveats." }],
    relatedLinks: [
      { title: "GPT generations expression evaluator", text: "Read the reported expression evaluator comparison and caveats.", href: "/benchmarks/gpt-generations-expression-evaluator" },
    ],
  },
  "/features/agent-replay": {
    editorial: { title: "What can a reviewer inspect?", example: "Use a run trajectory to investigate tool actions and contract failure.", limitation: "The current benchmark does not attach public replay artifacts." },
    proofPoints: ["Review tool actions and observations in a run.", "Compare the final artifact or contract with the task requirements."],
    workflow: [{ title: "Inspect the trajectory", text: "Review tool actions and observations in a run." }, { title: "Investigate the outcome", text: "Compare the final artifact or contract with the task requirements." }],
    relatedLinks: [
      { title: "Agent trajectory evaluation", text: "Learn what a trajectory can reveal.", href: "/agent-trajectory-evaluation" },
      { title: "CI/CD agent gates", text: "Configure and interpret a CI gate.", href: "/docs/guides/ci-cd-agent-gates" },
    ],
  },
  "/features/challenge-packs": {
    editorial: { title: "What does a pack accept and produce?", example: "Task, tools, execution policy and scoring in expr-eval-arena.yaml; resulting run supplies evaluation evidence.", limitation: "Exact fields and supported execution options depend on the parser" },
    proofPoints: ["Define the task, tools, execution policy and scoring in the pack.", "Run the workload and inspect the resulting evaluation evidence."],
    workflow: [{ title: "Author the inputs", text: "Define the task, tools, execution policy and scoring in the pack." }, { title: "Review the output", text: "Run the workload and inspect the resulting evaluation evidence." }],
    relatedLinks: [
      { title: "Write a challenge pack", text: "Author and validate a challenge pack.", href: "/docs/guides/write-a-challenge-pack" },
      { title: "Evaluation spec reference", text: "Review available evaluation configuration.", href: "/docs/challenge-packs/evaluation-spec-reference" },
    ],
  },
  "/synthetic-data-generation-agents": {
    editorial: { title: "When should I generate examples for eval or training?", example: "The synthetic guide contrasts Fast Self-Instruct and Agentic Self-Instruct, with DataSmith for training export.", limitation: "Generated rows need review and are not automatically representative" },
    proofPoints: ["Compare Fast Self-Instruct and Agentic Self-Instruct for your seeds.", "Review generated examples; use DataSmith when training export is the goal."],
    workflow: [{ title: "Choose a strategy", text: "Compare Fast Self-Instruct and Agentic Self-Instruct for your seeds." }, { title: "Review the candidates", text: "Review generated examples; use DataSmith when training export is the goal." }],
    relatedLinks: [
      { title: "Synthetic dataset generation", text: "Compare generation strategies and acceptance controls.", href: "/docs/guides/synthetic-dataset-generation" },
    ],
  },
  "/agentic-self-instruct": {
    editorial: { title: "How does weak/strong filtering work?", example: "Challenger proposes, solvers attempt, and policy filters based on scores; see the acceptance controls in the guide.", limitation: "Solver/judge selection and thresholds shape accepted rows" },
    proofPoints: ["A challenger proposes cases; weak and strong solvers attempt them.", "Set thresholds and inspect accepted and rejected candidates."],
    workflow: [{ title: "Propose and solve", text: "A challenger proposes cases; weak and strong solvers attempt them." }, { title: "Apply acceptance controls", text: "Set thresholds and inspect accepted and rejected candidates." }],
    relatedLinks: [
      { title: "Synthetic dataset generation", text: "Compare generation strategies and acceptance controls.", href: "/docs/guides/synthetic-dataset-generation" },
    ],
  },
  "/trace-to-dataset": {
    editorial: { title: "How can a trace become an eval candidate?", example: "Use OTel trace import as described in dataset/trace documentation, review and pin resulting cases.", limitation: "Trace content can contain sensitive data and does not automatically provide ground truth" },
    proofPoints: ["Use the documented OpenTelemetry trace import to create dataset candidates.", "Remove sensitive content, establish expected outcomes and pin useful cases."],
    workflow: [{ title: "Import a trace", text: "Use the documented OpenTelemetry trace import to create dataset candidates." }, { title: "Review and pin", text: "Remove sensitive content, establish expected outcomes and pin useful cases." }],
    relatedLinks: [
      { title: "Datasets overview", text: "Learn about dataset creation and trace imports.", href: "/docs/guides/datasets-overview" },
      { title: "Tools network and secrets", text: "Review tool, network and secret configuration.", href: "/docs/concepts/tools-network-and-secrets" },
    ],
  },
  "/features/synthetic-dataset-generation": {
    editorial: { title: "What are the inputs and outputs of a generation job?", example: "Seed dataset and selected strategy produce accepted/rejected candidates for review.", limitation: "Generation is not a substitute for curated coverage" },
    proofPoints: ["Choose a seed dataset and generation strategy.", "Inspect accepted and rejected candidates before adding cases to coverage."],
    workflow: [{ title: "Select inputs", text: "Choose a seed dataset and generation strategy." }, { title: "Review outputs", text: "Inspect accepted and rejected candidates before adding cases to coverage." }],
    relatedLinks: [
      { title: "Synthetic dataset generation", text: "Compare generation strategies and acceptance controls.", href: "/docs/guides/synthetic-dataset-generation" },
    ],
  },
  "/glossary/agentic-self-instruct": {
    editorial: { title: "What does the term mean?", example: "A challenger, weak and strong solvers, and a judge filter proposed examples.", limitation: "Passing a configured filter is not evidence of downstream improvement" },
    proofPoints: ["A challenger proposes an example for weak and strong solvers.", "A judge and configured thresholds determine whether the example is accepted."],
    workflow: [{ title: "Generate a candidate", text: "A challenger proposes an example for weak and strong solvers." }, { title: "Filter the result", text: "A judge and configured thresholds determine whether the example is accepted." }],
    relatedLinks: [
      { title: "Synthetic dataset generation", text: "Compare generation strategies and acceptance controls.", href: "/docs/guides/synthetic-dataset-generation" },
      { title: "Agentic Self-Instruct", text: "Explore the challenger and solver workflow.", href: "/agentic-self-instruct" },
    ],
  },
};
