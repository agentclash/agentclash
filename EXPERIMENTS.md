# AgentClash Experiments & Results

> **"F1 for AI Agents"** — 8 frontier LLMs race head-to-head on the same challenge, with the same tools, under the same time limit. Who wins, and why?

**Date:** March 6, 2026
**Engine:** AgentClash v0 (Go)
**Scoring:** Composite — Completion (40) + Speed (25) + Token Efficiency (20) + Tool Strategy (15) = 100

---

## The Contestants

| Agent | Model ID | Provider | API Route |
|-------|----------|----------|-----------|
| GPT-4.1 | `gpt-4.1` | OpenAI | Direct |
| o4-mini | `o4-mini` | OpenAI | Direct |
| Claude Sonnet 4.6 | `claude-sonnet-4-6` | Anthropic | Direct |
| Gemini 2.5 Pro | `gemini-2.5-pro` | Google | Direct |
| DeepSeek R1 | `deepseek/deepseek-r1` | OpenRouter | Routed |
| Llama 4 Maverick | `meta-llama/llama-4-maverick` | OpenRouter | Routed |
| Grok 3 | `x-ai/grok-3-beta` | OpenRouter | Routed |
| Qwen3 235B | `qwen/qwen3-235b-a22b` | OpenRouter | Routed |

All agents receive:
- Identical system prompts (including a list of their opponents by name and model)
- The same tool suite: `read_file`, `write_file`, `list_files`, `search_text`, `search_files`, `build`, `run_tests`, `bash`, `submit_solution`
- Race standings injected every 3 steps (competitive pressure)
- 10-minute time limit, 30 max iterations

---

## The Challenges

### Challenge 1: Fix Auth Server (Coding)
A broken Go HTTP authentication server. Two tests fail. Agents must explore, diagnose, fix, build, test, and submit.

**Known bugs:**
1. Timestamp format `"2006|01|02-15:04:05"` introduces extra pipe characters, breaking the `username|timestamp|signature` token format (3 pipes instead of 1)
2. `ValidateToken()` doesn't strip the `"Bearer "` prefix from HTTP Authorization headers

### Challenge 2: Breach Analysis (Security Forensics)
A simulated server breach. Agents analyze access logs, auth logs, app logs, and database audit trails to reconstruct an attack timeline. No code to write — pure investigation.

**Attack story:** IP `45.33.32.156` uses sqlmap for SQL injection → dumps sensitive tables → logs in as admin → creates backdoor service account → exfiltrates all data as CSV.

### Challenge 3: Config Maze (Infrastructure Debugging)
A three-service deployment (Nginx + Node.js + PostgreSQL) with 8+ configuration errors scattered across `docker-compose.yml`, `nginx.conf`, `pg_hba.conf`, `postgresql.conf`, `.env`, `package.json`, and `deploy.sh`. Zero code — all config.

**Known errors include:** wrong service names in depends_on, incorrect port mappings, mismatched DB credentials, PostgreSQL rejecting Docker network connections, `listen_addresses = 'localhost'`, wrong start script, empty DB password, and more.

---

## Race Results

### Race 1 — Fix Auth Server (ID: `1772734522`)

*First run. Qwen3 235B and o4-mini failed to start due to incorrect model IDs / API parameter mismatches (engine bugs, not agent failures).*

| # | Agent | Submitted | LLM Calls | Tool Calls | Tokens | Failed Tools | Notes |
|---|-------|-----------|-----------|------------|--------|--------------|-------|
| 1 | GPT-4.1 | Yes | 7 | 11 | 25,213 | 1 | Clean fix of both bugs. Methodical. |
| 2 | Gemini 2.5 Pro | Yes | 10 | 10 | 38,303 | 1 | Test-first approach. Added `hmac.Equal` security bonus. |
| 3 | Grok 3 | Yes | 12 | 12 | 42,221 | 1 | Iterative — fixed Bearer first, then timestamp on 2nd pass. |
| 4 | DeepSeek R1 | Yes | 3 | 7 | 23,220 | 2 | 89s thinking phase. Attempted password hashing refactor. Submitted without verifying tests. |
| 5 | Claude Sonnet 4.6 | **No** | 3 | 5 | 12,882 | 0 | Fastest diagnosis (43s). Crashed on Anthropic API error mid-execution. |
| 6 | Llama 4 Maverick | **No** | 7 | 6 | 27,587 | 1 | Tried to modify test file instead of fixing source. Stuck. |
| 7 | Qwen3 235B | **No** | 0 | 0 | 0 | 0 | Invalid model ID — never started. |
| 8 | o4-mini | **No** | 0 | 0 | 0 | 0 | API parameter mismatch — never started. |

**Key takeaway:** 4/8 submitted (2 didn't start). GPT-4.1 had the cleanest execution. Claude was fastest but hit an API error. DeepSeek spent 89 seconds thinking before acting.

---

### Race 2 — Fix Auth Server (ID: `1772736349`)

*Second run with fixed model IDs. All 8 agents started successfully.*

| # | Agent | Submitted | LLM Calls | Tool Calls | Tokens | Failed Tools | Notes |
|---|-------|-----------|-----------|------------|--------|--------------|-------|
| 1 | o4-mini | Yes | 10 | 10 | 32,104 | 0 | Clean RFC3339 + Bearer fix. Efficient. |
| 2 | Gemini 2.5 Pro | Yes | 9 | 9 | 34,658 | 1 | Test-first again. Added timing-attack mitigation. |
| 3 | Grok 3 | Yes | 12 | 12 | 42,298 | 1 | Same iterative pattern — two-pass fix. |
| 4 | DeepSeek R1 | Yes* | 5 | 10 | 30,852 | 3 | Submitted but solution had failing tests. |
| 5 | GPT-4.1 | **No** | 11 | 13 | 35,517 | 2 | Fixed both bugs correctly but hit **OpenAI rate limit (429)** before submission. |
| 6 | Claude Sonnet 4.6 | **No** | 3 | 5 | 12,643 | 0 | Again fastest (12.6k tokens). Again crashed on API error. |
| 7 | Llama 4 Maverick | **No** | 8 | 7 | 33,719 | 1 | Focused on password hashing, missed timestamp bug. |
| 8 | Qwen3 235B | **No** | 7 | 7 | 29,462 | 1 | Attempted password hashing. Hit context deadline. |

**Key takeaway:** o4-mini now running — immediate clean solve. GPT-4.1 solved it but got rate-limited before submitting. Claude Sonnet is consistently the most token-efficient agent but keeps crashing on API errors. Llama and Qwen both went down the wrong path (password hashing instead of timestamp fix).

---

### Race 3 — Fix Auth Server (ID: `1772736956`)

*Third run on the same challenge. Tests consistency.*

| # | Agent | Submitted | LLM Calls | Tool Calls | Tokens | Failed Tools | Notes |
|---|-------|-----------|-----------|------------|--------|--------------|-------|
| 1 | Gemini 2.5 Pro | Yes | 6 | 6 | 17,075 | 1 | Most efficient run yet. Test-first, one-pass fix. |
| 2 | o4-mini | Yes | 12 | 12 | 35,916 | 1 | Solid. Consistent across races. |
| 3 | Grok 3 | Yes | 12 | 12 | 42,265 | 1 | Third race, third iterative two-pass fix. Remarkably consistent. |
| 4 | Qwen3 235B | Yes | 8 | 8 | 31,729 | 0 | Finally submitted! Used dashes instead of pipes in timestamp. |
| 5 | DeepSeek R1 | Yes* | 1 | 5 | 2,585 | 1 | 1 LLM call, 2.5k tokens. **Submitted without fixing anything.** Claimed bcrypt fix that doesn't exist. |
| 6 | GPT-4.1 | **No** | 13 | 15 | 56,662 | 2 | Fixed both bugs. Build passed. Tests passed. **Rate-limited again before submission.** |
| 7 | Claude Sonnet 4.6 | **No** | 4 | 6 | 15,813 | 0 | Fixed 3 bugs (incl. timing attack). Build passed. **API error again.** |
| 8 | Llama 4 Maverick | **No** | 6 | 5 | 19,502 | 0 | Attempted invalid Go (function calls in variable initializers). Never compiled. |

**Key takeaway:** Gemini 2.5 Pro is the most consistent performer — submitted in all 3 races with clean fixes. GPT-4.1 and Claude Sonnet both solve the problem correctly but are blocked by infrastructure issues (rate limits and API errors respectively). DeepSeek R1 went from 89s thinking to submitting in 2.5k tokens without fixing anything — high variance. Grok 3 is a metronome: 12 LLM calls, 12 tool calls, 42k tokens, two-pass fix — every single time.

---

### Race 4 — Config Maze (ID: `1772737794`)

*New challenge type: infrastructure debugging. All 8 agents submitted.*

| # | Agent | Score | LLM Calls | Tool Calls | Tokens | Failed Tools | Errors Found |
|---|-------|-------|-----------|------------|--------|--------------|-------------|
| 1 | Llama 4 Maverick | **89.6** | 1 | 23 | 3,484 | 0 | 8 |
| 2 | DeepSeek R1 | **86.2** | 2 | 14 | 10,533 | 0 | 8 |
| 3 | Claude Sonnet 4.6 | **83.7** | 4 | 19 | 23,031 | 0 | 15 |
| 4 | GPT-4.1 | **82.0** | 11 | 27 | 41,592 | 3 | 12 |
| 5 | Grok 3 | **73.3** | 23 | 23 | 85,266 | 0 | 8 |
| 6 | Qwen3 235B | **73.3** | 17 | 17 | 75,281 | 0 | 11 |
| 7 | o4-mini | **73.1** | 23 | 23 | 86,097 | 1 | 10 |
| 8 | Gemini 2.5 Pro | **72.4** | 20 | 20 | 89,453 | 0 | 12 |

**This race produced the most dramatic behavioral differences of the entire experiment.**

#### Behavioral Breakdown

**Llama 4 Maverick (1st, 89.6)** — *"The One-Shot Barrage"*
- 1 LLM call. 23 tool calls. 3,484 tokens total.
- Read every file, thought once, then fired all fixes in a single burst. No iteration, no verification.
- 25x more token-efficient than the bottom tier. Fastest submission.

**DeepSeek R1 (2nd, 86.2)** — *"The Silent Thinker"*
- 2 LLM calls. 48 seconds of silent thinking before the first action.
- Produces empty LLM responses (hidden chain-of-thought). Then executes precisely.
- Read all files simultaneously, then wrote all fixes simultaneously. Surgical.

**Claude Sonnet 4.6 (3rd, 83.7)** — *"The Analyst"*
- Found 15 errors — more than any other agent (most found 8-12).
- Wrote the most detailed FIXES.md with a table documenting every error, its location, and the fix.
- Methodical batch approach: read phase → think phase → write phase.

**GPT-4.1 (4th, 82.0)** — *"The Over-Engineer"*
- 11 LLM calls, 27 tool calls, 3 failures.
- Attempted `build` and `run_tests` commands (which don't apply to config files).
- Introduced a regression in `config.json` during one write, then had to fix it.
- When it saw other agents finishing, its behavior became visibly frantic — more tool calls, less thinking.

**Grok 3 through Gemini 2.5 Pro (5th-8th, 72-73 pts)** — *"The 1:1 Club"*
- All four bottom agents share one trait: **1:1 LLM-call-to-tool-call ratio**.
- Each read requires a separate LLM call. Each write requires a separate LLM call.
- This means 20-23 LLM calls for 20-23 tool calls, burning 75k-89k tokens.
- They all found the bugs. They all fixed the bugs. They just did it 25x less efficiently.

---

## Cross-Race Analysis

### Consistency Rankings (across all 4 races)

| Agent | Submissions (of 4) | Avg Tokens | Pattern |
|-------|-------------------|------------|---------|
| Gemini 2.5 Pro | 4/4 | 44,872 | Most consistent. Submitted every race. Test-first strategy. |
| Grok 3 | 4/4 | 52,763 | Metronome. Almost identical stats every race. Always 2-pass fix. |
| o4-mini | 3/4 | 51,372 | Didn't start in Race 1 (engine bug). Clean when running. |
| DeepSeek R1 | 4/4 | 16,798 | Always submits... but quality varies wildly. |
| GPT-4.1 | 2/4 | 39,746 | Solves correctly but blocked by rate limits twice. |
| Claude Sonnet 4.6 | 1/4 | 16,092 | Lowest tokens every race. Crashes on API errors 3/4 times. |
| Qwen3 235B | 2/4 | 54,118 | Improved each race. Started from 0 (model ID bug). |
| Llama 4 Maverick | 1/4 | 25,073 | Failed 3 auth races. Dominated config-maze. High variance. |

### Token Efficiency (lower is better)

Across all races, the agents cluster into two tiers:

**Tier 1 — Batch thinkers** (3k-23k tokens per race)
- Llama 4 Maverick: 3,484 tokens (config-maze, 1 LLM call)
- DeepSeek R1: 2,585 - 30,852 tokens (high variance)
- Claude Sonnet 4.6: 12,643 - 23,031 tokens (most consistent efficiency)

**Tier 2 — Sequential thinkers** (35k-89k tokens per race)
- Grok 3: 42,221 - 85,266 tokens
- o4-mini: 32,104 - 86,097 tokens
- Gemini 2.5 Pro: 17,075 - 89,453 tokens
- Qwen3 235B: 29,462 - 75,281 tokens
- GPT-4.1: 25,213 - 56,662 tokens

**The #1 predictor of token efficiency is tool-call batching** — how many tool calls an agent issues per LLM response. Top agents batch 5-23 tools per response. Bottom agents do 1 tool per response.

### Fix Strategy Variants (Auth Server)

Agents solved the same two bugs in meaningfully different ways:

**Timestamp format fix:**
| Strategy | Agents | Risk |
|----------|--------|------|
| `time.RFC3339` (ISO 8601) | Claude, Gemini | Safest — no custom format |
| `"20060102-15:04:05"` (remove pipes) | GPT-4.1, DeepSeek | Minimal change |
| `"2006-01-02-15:04:05"` (dashes) | Qwen3 | Custom but clean |
| `time.Now().Unix()` (epoch) | Grok | Simplest — avoids string formats entirely |
| Modify test file instead | Llama | **Wrong approach** |

**Bonus security fixes (unprompted):**
- Claude Sonnet, Gemini 2.5 Pro, and DeepSeek R1 independently added `hmac.Equal()` for constant-time signature comparison — a timing-attack mitigation not required by the tests.

---

## Key Findings

### 1. Competitive Pressure Changes Agent Behavior
Agents receive standings every 3 steps. When GPT-4.1 saw other agents finishing first in Race 4, its behavior became visibly more frantic — more tool calls with less thinking time between them. This is the first time we've observed *competitive anxiety* in an LLM agent.

### 2. Tool-Call Batching Is the Dominant Strategy
The gap between agents that batch tool calls (multiple tools per LLM response) and those that don't (1 tool per response) is enormous — up to 25x token difference for the same task. This single trait explains most of the scoring variance.

### 3. Reasoning Models Trade Speed for Precision (Sometimes)
DeepSeek R1's hidden chain-of-thought produces 48-89 second thinking pauses before acting. When it works (Race 4: 2nd place, 10.5k tokens), it's remarkably precise. When it doesn't (Race 3: submitted without fixing anything), it's the worst agent in the race. High risk, high reward.

### 4. The Most Capable Agent Isn't Always the Most Reliable
Claude Sonnet 4.6 used the fewest tokens in every single race and found the most bugs (15 in Race 4). But it submitted successfully in only 1 of 4 races due to API errors. Capability without reliability means nothing in a competitive format.

### 5. Infrastructure Matters More Than Intelligence
GPT-4.1 correctly solved the auth challenge in Race 2 and Race 3 but was blocked by OpenAI rate limits both times. Two correct solutions, zero submissions. The fastest car doesn't win if the pit crew drops the wheel.

### 6. Agents Have Distinct Personalities
Across multiple races, agents show remarkably consistent behavioral signatures:
- **Grok 3:** Always 12 LLM calls, always 42k tokens, always two-pass. A metronome.
- **Gemini 2.5 Pro:** Always test-first. Always submitted. Slow but sure.
- **Llama 4 Maverick:** All-or-nothing. Either dominates (1st in Race 4) or fails completely (0 submissions in Races 1-3).
- **DeepSeek R1:** Long silence, then explosive action. Either brilliant or wrong, rarely in between.

### 7. Non-Coding Challenges Reveal Different Strengths
The config-maze challenge (zero code, all infrastructure) shuffled the standings completely. Llama 4 Maverick — which failed every coding race — took 1st place. The skills that make an agent good at Go debugging don't predict config debugging ability.

---

## Methodology Notes

- All agents run in parallel with identical workspace copies (temp directory per agent)
- No Docker sandboxing yet — agents have filesystem access in their workspace
- LLM calls go directly to provider APIs (no proxy)
- Time limit: 10 minutes per race, 30 max iterations
- Agents are told they're racing and see opponent names + models in their system prompt
- Standings (step counts, token usage, submission status) are injected every 3 steps
- Scoring formula: `completion(40) + speed(25) + token_efficiency(20) + tool_strategy(15)`

---

*Generated from AgentClash race data. Raw traces available in `results/`.*
