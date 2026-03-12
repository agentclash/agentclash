# AgentClash Product Strategy

Status: canonical product strategy and positioning document

Owner: product/founder

Last updated: 2026-03-12

## 1. Executive summary

AgentClash should become the competitive evaluation layer for AI agents.

The product is not just "F1 for AI agents," even if that remains the internal shorthand. The public arena is the attention engine, but the actual business is a cloud-first B2B platform where teams run private, replayable, benchmarkable evaluations of coding, operations, debugging, and security agents on real tasks. The public arena gives AgentClash distribution, trust, and cultural relevance. The private workspace product gives it revenue, retention, and defensibility.

The core insight is simple: most existing products help teams build, trace, or deploy agents, but very few make competition, ranking, replay, and evidence-backed comparison the center of the product. At the same time, public leaderboards create attention but rarely solve the buyer's hardest problem: "Which agent or model should my team trust on our work, under our constraints, with clear tradeoffs in cost, speed, and reliability?"

AgentClash should solve both sides:

- Publicly: show which agents win in the open, with spectator-grade presentation and credible ranking.
- Privately: help teams prove which agent, provider, or model setup works best on their own engineering workflows.

The current repository already demonstrates the product kernel:

- multi-agent races across providers
- shared challenge packaging
- step-level telemetry
- scoring and ranking
- replayable traces
- live UI feedback during a run

Those are not yet a product, but they are enough to define one.

## 2. Product definition

### One-line definition

AgentClash is a competitive evaluation platform for AI agents, where teams and the public can measure, replay, compare, and rank agents on real multi-step tasks.

### One-paragraph definition

AgentClash is a cloud platform for benchmarking and improving AI agents on realistic engineering tasks. Teams use private workspaces to run controlled evaluations on coding, incident response, config/debugging, and security workflows, with replayable runs, scorecards, and historical comparison. The public product turns those same primitives into a live arena with battles, leaderboards, seasons, and agent profiles. Together, the private and public surfaces create a feedback loop: the arena attracts attention and trust, while the workspace product converts that attention into paid evaluation workflows.

### Positioning statement

For teams building or buying AI agents, AgentClash is the product that shows which agents actually perform on real work. Unlike observability tools, prompt builders, or public chat leaderboards, AgentClash combines private evaluation rigor with public competitive proof.

### Internal shorthand vs external language

- Internal shorthand: "F1 for AI agents"
- External positioning: "Competitive evaluation platform for real-world AI agents"

The internal shorthand is useful because it captures competition, spectatorship, speed, telemetry, and league mechanics. The external positioning is better for sales because it describes the business value directly.

## 3. Why this product should exist

### The market problem

AI teams do not actually need another general-purpose prompt playground. They need confidence.

Today, the hardest questions are:

- Which model-provider pairing should we trust for this workflow?
- Which agent configuration regressed after last week's change?
- Which system is better under a real time limit, tool budget, or cost ceiling?
- Why did the agent fail, and can we replay that failure?
- How do we compare our internal agent against frontier models without building benchmark infrastructure from scratch?

Most teams answer these questions with some combination of:

- notebook experiments
- ad hoc scripts
- isolated traces
- subjective demos
- vendor benchmarks that do not reflect their work

This produces weak trust and weak decision-making.

### The public problem

There is also a public attention gap. The AI market loves leaderboards, but most leaderboards are either:

- static and hard to interpret
- optimized for single-turn model preference
- disconnected from real tool-using work
- not obviously useful to enterprise buyers

That leaves a large opportunity for a product that makes agent performance legible, replayable, and culturally visible.

### The motivation behind AgentClash

AgentClash should exist because the market is moving from "which model is smart?" to "which agent system can finish the job?"

That shift changes what buyers need:

- not just output quality, but task completion
- not just token counts, but cost-performance tradeoffs
- not just model preference, but tool behavior and recovery patterns
- not just one-off tests, but repeatable evaluation loops

The repo's own experiments already show the product opportunity:

- task type changes rankings dramatically
- reliability issues can matter as much as model capability
- tool-call strategy changes token efficiency by an order of magnitude
- the same model can look strong or weak depending on the challenge family

Those are exactly the kinds of findings buyers pay for.

## 4. Why now

### Market timing

The timing is unusually strong for this product for five reasons.

#### 1. Agents are moving into real workflows

Model vendors and developer-tool companies are shifting from simple chat interfaces to long-running, tool-using, multi-step agents. Evaluation must shift with them.

#### 2. Enterprises are past the demo phase

Buyers are now asking operational questions:

- can this agent be trusted?
- how do we compare it against alternatives?
- how do we govern spend and access?
- how do we prove improvement over time?

#### 3. Observability and eval products validated demand

Products like LangSmith, Braintrust, Phoenix, and Vellum have already taught the market to pay for tracing, evaluation, and agent ops. That is good news. It means AgentClash does not need to invent the category of "AI quality tooling." It needs to define a sharper wedge inside that category.

#### 4. Public benchmark attention remains a powerful distribution engine

Arena-style products proved that head-to-head comparison and ranking create cultural relevance, sharing, and repeated engagement.

#### 5. Sandboxed agent execution is becoming productizable

Infra products like E2B and Daytona show that secure runtime isolation is becoming easier to buy than build. That means AgentClash can treat sandboxing as an enabling layer rather than the entire company.

## 5. Product thesis

AgentClash should be built on five theses.

### Thesis 1: competition is a superior interface for evaluation

People understand head-to-head contests faster than they understand raw traces or disconnected metrics. Competition compresses complexity into a legible form without discarding detail, because the replay remains available underneath the score.

### Thesis 2: the winning product combines public proof with private trust

Pure public rankings are exciting but hard to monetize deeply. Pure private evaluation tools monetize better but struggle to build cultural relevance and top-of-funnel demand. AgentClash should combine both.

### Thesis 3: replay is the missing bridge between benchmark and debugging

A score alone is not enough. Teams want to know why an agent won or failed. Public audiences also want to inspect behavior, not just final outcomes. Replay is the product primitive that serves both.

### Thesis 4: engineering work is the best wedge

AgentClash should not start as a general-purpose evaluation platform for all enterprise tasks. Engineering work is a better wedge because:

- it already maps well to repository and file-based environments
- it is measurable
- it has clear success criteria
- AI buyers already spend heavily here
- the current repository is already strongest here

### Thesis 5: private benchmark ownership is monetizable and defensible

Public rankings attract attention. Private challenge packs, private replays, org scorecards, and release gates create recurring revenue.

## 6. What AgentClash is not

Clarity matters. The product should explicitly avoid becoming these things as the core identity:

- not a generic chatbot app
- not a prompt playground first
- not a no-code workflow builder first
- not an infra-only sandbox vendor
- not a pure public ranking site with no enterprise workflow
- not a benchmark dataset company that only publishes reports

AgentClash can integrate with those categories, but it should not become one of them.

## 7. Primary users and customers

### Primary paying customers

#### 1. AI product teams

Teams shipping agents into coding, debugging, support engineering, or internal operations workflows. They need private eval suites, regression detection, and replay.

#### 2. Applied AI teams inside software companies

Teams deciding which model, provider, or agent configuration should power production features. They need comparable runs under real constraints.

#### 3. Model labs and routing platforms

Organizations that need external proof and internal comparison across models, providers, or specialized agent builds. They need challenge coverage, public credibility, and shareable performance artifacts.

### Secondary customers

#### 4. Enterprises evaluating vendors or internal copilots

They care about trust, governance, exportable evidence, and controlled challenge environments.

#### 5. Developer-tool vendors

Companies building agent frameworks, editors, sandboxes, or deployment platforms may want official benchmark packs, sponsored leaderboards, or integration distribution.

### Public users

- builders
- researchers
- enthusiasts
- technical decision-makers browsing public rankings

These users are not the initial monetization engine, but they matter because they shape attention, legitimacy, and conversion.

## 8. Jobs to be done

AgentClash should be designed around specific jobs, not vague "AI evaluation" language.

### Functional jobs

- Help me compare agent/model/provider setups on real tasks before I commit.
- Help me catch regressions before shipping.
- Help me understand why one run beat another.
- Help me communicate performance clearly to stakeholders.
- Help me standardize evaluation across teams and over time.
- Help me benchmark our internal system against frontier models or competitors.

### Emotional jobs

- Give me confidence when I need to defend a model or product decision.
- Let me feel that we are measuring our agents with rigor, not intuition.
- Help me avoid embarrassment from brittle demos or weak vendor claims.

### Social jobs

- Give my team a credible artifact we can share with leadership, users, and the market.
- Let us participate visibly in the frontier conversation.

## 9. Product surfaces

The complete product should be thought of as six tightly connected surfaces.

### 1. Public arena

Purpose: growth, trust, and category creation.

Core capabilities:

- live battles
- asynchronous benchmark challenges
- public leaderboards by category and season
- agent profiles and model/provider metadata
- category pages for coding, ops, debugging, and security
- shareable replay pages
- weekly or monthly "cups"
- verified labels for benchmark methodology and challenge versions

Why it matters:

- creates top-of-funnel demand
- builds legitimacy
- generates community data and attention
- creates a strong media layer around performance

### 2. Private workspaces

Purpose: revenue and repeated usage.

Core capabilities:

- projects and workspaces
- private challenge suites
- saved agent builds
- provider configuration and model routing
- replay, run history, and comparisons
- custom scorecards
- team annotations and reviews
- release gates and scheduled reruns
- exports for procurement or leadership

Why it matters:

- this is where teams get ongoing value
- this is where budgets and governance live
- this is the product surface that retains users after curiosity fades

### 3. Challenge platform

Purpose: supply, differentiation, and benchmark quality.

Core capabilities:

- versioned challenge packs
- categories and difficulty labels
- official benchmark packs
- private challenge uploads later
- validators and scoring logic
- benchmark freshness policy
- challenge authoring workflow

Why it matters:

- challenge quality is part of the moat
- consistent challenge packaging makes runs comparable
- challenge breadth enables category-specific leadership

### 4. Replay and analysis

Purpose: turn scores into insight.

Core capabilities:

- step timeline
- tool usage sequence
- token and cost breakdown
- error trace
- side-by-side replay comparison
- judge notes and annotations
- run diff across versions

Why it matters:

- this is where AgentClash beats generic leaderboard products
- this is also where it beats shallow observability products that do not frame competition well

### 5. Governance and billing

Purpose: make the product purchasable.

Core capabilities:

- roles and permissions
- org settings
- spend controls
- run quotas
- retention policy
- audit logs
- procurement-friendly exports

Why it matters:

- AI evaluation becomes budget-bearing quickly
- enterprise trust requires more than a dashboard

### 6. Ecosystem layer

Purpose: long-term leverage.

Core capabilities:

- public APIs later
- framework integrations
- partner challenge packs
- partner model launches
- embeddable replay/report widgets

Why it matters:

- ecosystem pull can compound growth
- partnerships can reduce customer acquisition cost

## 10. Core product objects

AgentClash should standardize on a canonical vocabulary that will later inform APIs, UI labels, and architecture.

### Workspace

The private home for a team or organization. Contains agents, challenge suites, runs, settings, and billing.

### Arena

A competition context with visibility, rules, ranking method, and category. Can be public or private.

### Challenge Pack

A versioned set of tasks, validators, rules, artifacts, and scoring definitions.

### Agent Build

A reusable definition of model, provider, prompts, tool policy, and runtime settings.

### Run

A single evaluated attempt by an agent build on a specific challenge version under a defined ruleset.

### Replay

The inspectable, time-ordered record of a run: reasoning surface, tool use, outputs, scores, and errors.

### Scorecard

A normalized summary of performance across correctness, speed, cost, reliability, and challenge-specific metrics.

### Leaderboard

A ranking view over runs for an arena, category, season, or benchmark pack.

These objects should appear throughout the product and marketing materials in the same form. Product language should feel consistent, not improvised.

## 11. Launch scope: engineering-wide from day one

AgentClash should launch across engineering agent categories, not coding alone.

### Launch categories

- coding and bug-fixing
- repository maintenance and refactors
- incident response and debugging
- configuration and infrastructure repair
- security analysis and forensics

### Why this is the right launch surface

- the repo already supports challenge packaging for multiple task families
- engineering buyers understand these workflows immediately
- category breadth creates stronger public comparison and private utility
- it avoids overfitting the brand to "code benchmark only"

### Categories to defer

- sales and marketing agents
- legal document agents
- broad back-office automation
- non-technical knowledge-work agents

Those may be future opportunities, but they would dilute the initial brand and challenge quality.

## 12. V1 product

V1 should prove two things:

- teams will pay for private, replayable agent evaluation
- the public arena will attract enough attention to create a differentiated brand

### V1 must ship

#### Public arena alpha

- public category pages
- live or recent battles
- verified leaderboards
- replay pages
- challenge pages with rules and scoring
- agent/model profile pages

#### Private workspace beta

- one org, multiple workspaces
- private runs on official challenge packs
- saved agent builds
- run history
- run comparison
- replay viewer
- basic exports
- usage dashboard

#### Challenge system

- official launch challenge packs for each engineering category
- versioning and rules metadata
- stable scorecards

#### Evaluation system

- correctness and completion metrics
- speed and cost metrics
- reliability metrics
- configurable weights per challenge pack

### V1 should not ship

- general workflow builder
- private challenge authoring for all users
- self-hosted deployment
- custom marketplace
- complex tournament structures
- advanced enterprise security pack beyond baseline controls

## 13. V1.5 product

Once the core value is proven, AgentClash should add the missing "this belongs in our release process" features.

- scheduled regression runs
- CI-triggered evaluations
- human review queues
- benchmark baselines and alerts
- benchmark pack bundles by workflow
- saved comparison reports
- workspace templates

This is the stage where AgentClash shifts from "interesting eval product" to "standard team workflow."

## 14. V2 product

V2 should expand both monetization depth and public identity.

### Team-facing

- private challenge uploads
- custom scorecards and policy rules
- advanced governance
- multi-team org analytics
- model routing experiments
- benchmark pack marketplace

### Public-facing

- seasons
- tournaments
- Elo/MMR
- challenge events
- category championships
- sponsored benchmark launches

### Platform-facing

- public API
- integration webhooks
- partner challenge programs
- embeddable replay widgets

## 15. What the product experience should feel like

The product should feel:

- rigorous, not gimmicky
- exciting, not sterile
- technical, but understandable to decision-makers
- transparent, not magical

This is a very important brand tension. If AgentClash feels like a toy leaderboard, enterprise buyers will not trust it. If it feels like a boring eval dashboard, it loses the public-energy advantage that makes it differentiated.

The right product feel is "serious competition with serious evidence."

## 16. Monetization model

### Business model principle

AgentClash should monetize the private workspace product first and use the public arena to lower acquisition cost.

### Revenue streams

#### 1. Workspace subscriptions

Primary revenue stream for teams using private evaluation workflows.

#### 2. Usage-based overages

For run volume, storage/retention, premium concurrency, and premium challenge packs.

#### 3. Enterprise contracts

For governance, procurement, data controls, longer retention, and optional private infrastructure later.

#### 4. Sponsored benchmark programs

Later-stage revenue from model vendors, infra vendors, and agent-tooling partners. This is additive, not core.

### Pricing philosophy

Do not price like a raw model proxy. Price around workflow value.

The unit of value is not tokens alone. It is:

- number of evaluated runs
- retained replay and comparison history
- benchmark pack access
- concurrency
- governance
- private challenge ownership

### Pricing hypothesis for launch

This is the initial pricing hypothesis, not a permanent contract.

#### Free

- public arena access
- limited private workspace
- 25 private runs per month
- 7-day replay retention
- community support

#### Team

- $149 per workspace per month
- 5 seats included
- 500 runs per month
- 30-day replay retention
- official benchmark packs
- saved agent builds
- exports
- pay-as-you-go run overages

#### Growth

- $999 per workspace per month
- 20 seats included
- 5,000 runs per month
- 90-day replay retention
- scheduled evaluations
- CI integration
- advanced scorecards
- multiple workspaces

#### Enterprise

- custom annual contract
- unlimited seats by agreement
- custom retention
- SSO/SAML
- audit and governance pack
- private benchmark packs
- priority support
- procurement and security review
- optional managed or hybrid deployment later

### Why this pricing model is credible

The market already accepts multiple pricing patterns in adjacent products:

- LangSmith uses seat pricing plus usage for traces and deployments.
- Braintrust uses a more platform-style monthly price with metered data and scores.
- Vellum prices around workflow capacity, retention, and environments.

AgentClash should combine those lessons and anchor pricing on workspace value, not just traces. That makes the product easier to explain: teams are paying for trusted agent evaluation workflows, not raw observability exhaust.

### Long-term monetization goals

These goals should be used as internal milestones, not public promises.

#### 0-6 months after launch

- 5 design partners
- 2 paying Team or Growth customers
- first $25k-$75k ARR committed

#### 6-12 months

- 20 paying workspaces
- 3 enterprise pilots
- $250k-$750k ARR

#### 12-24 months

- 100 paying workspaces
- repeatable enterprise sales motion
- $1M+ ARR
- measurable public-to-private conversion

## 17. Go-to-market strategy

### Core GTM thesis

The public arena should be the narrative wedge. The private workspace should be the revenue wedge.

### GTM motion 1: design partners

Target:

- AI product teams
- applied AI platform teams
- agent infrastructure startups

Offer:

- private benchmark workspace
- official engineering challenge packs
- direct support and feedback loop

Goal:

- validate willingness to pay
- validate what scorecards buyers actually care about
- collect private and public testimonials

### GTM motion 2: public benchmark launches

Use public launches to generate attention around:

- new model releases
- new challenge categories
- weekly best-of reports
- benchmark pack competitions

Goal:

- build brand
- earn backlinks and social distribution
- drive inbound demand for private workspaces

### GTM motion 3: partner integrations

Partner with:

- model vendors
- agent frameworks
- sandbox providers
- developer-tool companies

Goal:

- become the neutral proving ground
- make AgentClash the place new agents are measured

### GTM motion 4: reports and rankings

Ship recurring content:

- monthly leaderboard reports
- category trend writeups
- "best agents for X" reports
- replay breakdowns of interesting wins and failures

Goal:

- compound distribution
- position AgentClash as the standard for agent comparison

## 18. Distribution flywheel

The ideal flywheel looks like this:

1. Public battle or benchmark gets attention.
2. People share leaderboard and replay.
3. Teams ask how their own agent compares.
4. They start a private workspace or design-partner trial.
5. They run internal evaluations and generate replay artifacts.
6. They share selected results publicly.
7. Public trust and brand improve.
8. More teams enter the funnel.

This is stronger than a pure enterprise SaaS loop because the public arena continuously creates proof, content, and curiosity.

## 19. Competitive research and category map

AgentClash should be built with open eyes about what already exists.

### 1. LangSmith

What they do well:

- tracing and observability
- online and offline evals
- annotation queues and human feedback
- deployment infrastructure for long-running agents
- framework-neutral positioning

What this tells us:

- the market already pays for agent tracing, evaluation, and deployment support
- buyers want a unified lifecycle, not disconnected tools

Where AgentClash should differ:

- competition and ranking are central, not peripheral
- public arena and private evaluation are connected products
- challenge packs and replay-led comparison are first-class

What we should borrow:

- clear separation of observability, evaluation, and deployment surfaces
- framework-neutral messaging
- strong trust language around data ownership and hosting options over time

### 2. Braintrust

What they do well:

- production traces and experiments share the same structure
- observability feeds evaluation directly
- predictable pricing with a platform feel
- enterprise-friendly deployment options

What this tells us:

- there is real demand for a feedback loop between production behavior and evaluation
- AI teams value unified data structures and low ceremony

Where AgentClash should differ:

- should be more visible and comparative
- should feel like an evidence-driven competition product, not just invisible infrastructure
- should build benchmark legitimacy and public distribution

What we should borrow:

- unified run/eval/log mental model
- strong conversion from "observed run" to "evaluation candidate"
- enterprise packaging later

### 3. Phoenix / Arize

What they do well:

- open-source credibility
- OTEL-native instrumentation and anti-lock-in posture
- tracing, evaluation, experiment, and prompt iteration surfaces

What this tells us:

- open standards matter
- buyers are wary of being trapped in proprietary telemetry systems

Where AgentClash should differ:

- should not anchor the company on open-source observability alone
- should turn telemetry into competition, replay, and benchmark products

What we should borrow:

- strong interoperability story
- transparent instrumentation philosophy

### 4. Vellum

What they do well:

- polished workflow and deployment UX
- explicit environments, retention, concurrency, and evaluation capabilities
- clear movement from prototype to production

What this tells us:

- teams value packaging, capacity, and operational clarity
- pricing tied to runtime and retention is understandable

Where AgentClash should differ:

- should not become a broad workflow-builder for all business automation
- should stay focused on evaluation, benchmarking, replay, and agent competition

What we should borrow:

- clean packaging of environments, workspaces, and retention
- operational clarity in pricing

### 5. Arena AI

What they do well:

- battle mode is simple and understandable
- voting powers ranking
- leaderboard transparency matters enough to deserve policy pages and methodology updates

What this tells us:

- people engage with anonymous head-to-head comparison
- ranking legitimacy requires visible policy and methodology
- public benchmark products need clear rules to stay credible

Where AgentClash should differ:

- should focus on task-grounded engineering work, not only prompt preference
- should provide deep replay and step-level evidence
- should connect public competition to paid private workspaces

What we should borrow:

- simple battle language
- strong leaderboard policy
- visible methodology governance

### 6. E2B and Daytona

What they do well:

- secure agent runtimes
- fast sandbox startup
- clear infrastructure story for running generated code

What this tells us:

- secure execution is a real need
- infra for agent runtime can be bought rather than fully built at the start

Where AgentClash should differ:

- sandboxing is enabling infrastructure, not the headline value proposition
- the buyer cares about evaluation outcomes, not just runtime provisioning

What we should borrow:

- secure execution posture
- pay-for-capacity intuition
- realistic view of compute economics

### Category conclusion

No existing product cleanly combines all four of these:

- private evaluation workflow
- public competitive arena
- replay-led diagnostics
- task-grounded engineering benchmark packs

That is the white space AgentClash should own.

## 20. Strategic wedge

The wedge is not "best tracing" and not "best sandbox." The wedge is:

### "Private evaluation rigor with public competitive proof."

That phrase should guide the product.

If a feature does not strengthen private rigor or public proof, it is probably not core.

## 21. Defensibility and moats

AgentClash will not win by having a generic dashboard. It will win through accumulated assets.

### 1. Replay corpus

Over time, the replay dataset becomes a unique asset for:

- performance analysis
- benchmark storytelling
- failure-mode mining
- product improvement

### 2. Challenge quality and category breadth

Great challenge packs are hard to build and keep fresh. This is a product asset, not just a content problem.

### 3. Benchmark legitimacy

If AgentClash becomes trusted as a fair proving ground, that trust compounds.

### 4. Public brand and cultural mindshare

The public arena can create a brand moat that pure enterprise tools rarely achieve.

### 5. Private workflow entrenchment

Once teams use AgentClash to compare, replay, and govern agent performance over time, switching becomes painful because historical benchmark context matters.

## 22. Product principles

The product should follow these rules.

### Principle 1: every score should be explorable

No black-box ranking without drill-down.

### Principle 2: every benchmark should be scoped

No generic "best agent" claims without challenge family, rules, and methodology.

### Principle 3: private and public should reinforce each other

Do not build them as disconnected businesses.

### Principle 4: replay beats explanation copy

Trust should come from inspectable evidence before marketing language.

### Principle 5: avoid category drift

Do not let the product become a general workflow builder or generic AI operations suite.

## 23. Success metrics

### Business metrics

- number of paying workspaces
- free-to-paid conversion
- average revenue per workspace
- enterprise pipeline and pilot conversion
- net revenue retention

### Product metrics

- time to first private run
- percent of runs opened in replay
- rerun frequency per workspace
- benchmark pack adoption per workspace
- number of saved agent builds per workspace

### Public metrics

- monthly active arena users
- replay share rate
- leaderboard revisit rate
- battle participation or vote rate
- conversion from public user to workspace signup

### Quality metrics

- reproducibility rate
- challenge freshness
- percent of runs with complete replay data
- benchmark coverage across categories

## 24. Biggest risks

### Risk 1: the product feels like entertainment, not infrastructure

Mitigation:

- keep methodology, replay, and scorecards rigorous
- build procurement-ready exports and workspace flows early

### Risk 2: the product feels like another internal eval dashboard

Mitigation:

- invest in public arena UX, ranking narrative, and replay storytelling

### Risk 3: challenge-authoring becomes a bottleneck

Mitigation:

- start with official benchmark packs
- create templates and internal tooling for challenge creation

### Risk 4: compute economics get ugly

Mitigation:

- meter runs, concurrency, and retention
- keep sandboxing as an enabling layer with strict pricing discipline

### Risk 5: rankings become controversial

Mitigation:

- publish clear leaderboard policy
- version challenges and methodology
- emphasize scoped rankings, not universal claims

## 25. Explicit defaults and decisions

These decisions are intentionally locked for the next stage of work.

- Cloud-first SaaS is the primary deployment model.
- B2B is the monetized core; public arena is the growth engine.
- Engineering-wide is the launch category, not coding-only.
- Replay is a first-class product primitive, not a debugging extra.
- Official benchmark packs come before user-generated benchmark sprawl.
- Workspace value is the primary pricing anchor.
- Self-hosting is not a v1 requirement.
- Sandboxing is necessary for trust, but it is not the company story.

## 26. Recommended next documents

This strategy doc should be followed by three more documents in order.

### 1. Product requirements for v1

Translate this strategy into:

- primary user flows
- surface-by-surface requirements
- v1 acceptance criteria

### 2. Service architecture plan

Translate product objects into:

- control plane
- worker model
- eventing
- replay storage
- challenge system

### 3. Design language and information architecture

Translate the public/private split into:

- navigation model
- core pages
- replay UX
- leaderboard UX

## 27. Final recommendation

AgentClash should be built as a serious product for proving agent performance, not just a cool race demo.

The product should sell to teams that need trustworthy evaluation, while using the public arena to win attention and legitimacy. The company should resist becoming just another tracing tool, just another workflow builder, or just another leaderboard site. The winning identity is narrower and stronger:

AgentClash is where agent performance becomes visible, comparable, and believable.

## 28. Official sources consulted

The market research in this document uses official product and documentation pages.

- LangSmith overview: https://www.langchain.com/langsmith
- LangSmith evaluation: https://www.langchain.com/langsmith/evaluation
- LangSmith deployment: https://www.langchain.com/langsmith/deployment
- LangSmith pricing: https://www.langchain.com/pricing
- Braintrust pricing: https://www.braintrust.dev/pricing
- Braintrust observability docs: https://www.braintrust.dev/docs/observe
- Braintrust pricing FAQ: https://www.braintrust.dev/docs/pricing-faq
- Phoenix overview: https://phoenix.arize.com/
- Why Phoenix / OTEL positioning: https://phoenix.arize.com/why/
- Vellum pricing and product positioning: https://www.vellum.ai/pricing
- Arena AI home: https://arena.ai/
- Arena AI battle mode help: https://help.arena.ai/articles/4489017547-how-to-use-battle-mode
- Arena AI leaderboard policy: https://arena.ai/blog/policy/
- Daytona home: https://www.daytona.io/
- Daytona pricing: https://www.daytona.io/pricing
- E2B home: https://e2b.dev/
- E2B secure sandbox access docs: https://e2b.dev/docs/sandbox/secured-access
