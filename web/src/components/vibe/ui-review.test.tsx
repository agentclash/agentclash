import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { readFileSync } from "node:fs";
import { beforeEach, afterEach, expect, it, vi } from "vitest";
import { BuildStart } from "./build-start";
import { ImproveStart } from "./improve-start";
import { ModelSelect } from "./model-select";
import { EvaluationEntry } from "./evaluation-entry";
import { PrototypeTrial } from "./prototype-trial";
import { VibeDataUseNotice } from "./vibe-data-use";
import { ChevronRight } from "lucide-react";

let node: HTMLDivElement, root: Root, style: HTMLStyleElement;
const noop = vi.fn();
const build = { draft: "", onDraft: noop, price: { kind: "trial" as const }, trialUsed: false, busy: false, onStart: noop, onSwitchDoor: noop, onBack: noop };
const improve = { mode: "instructions" as const, onMode: noop, drafts: { instructions: "", conversations: "", rules: "" }, onDraft: noop, models: [{ id: "openai/gpt-4.1-mini", name: "openai/gpt-4.1-mini" }], signedIn: false, busy: false, onWriteTests: noop, onSaveReplies: noop, onImportPack: noop, onOther: noop, onSwitchDoor: noop, onBack: noop };
const css = readFileSync("src/components/vibe/project-workspace.css", "utf8");
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  node = document.createElement("div"); node.className = "vibe-workspace"; document.body.append(node); root = createRoot(node);
  style = document.createElement("style"); style.textContent = ["workspace.css", "build-output.css", "improve.css", "project-workspace.css"].map(file => readFileSync(`src/components/vibe/${file}`, "utf8")).join("\n"); document.head.append(style);
});
afterEach(async () => { await act(async () => root.unmount()); node.remove(); style.remove(); vi.unstubAllGlobals(); });
const draw = (view: React.ReactNode) => act(async () => root.render(view));
it("U1: both starts use exactly one H1 with the same serif display role", async () => {
  for (const screen of [<BuildStart key="build" {...build} />, <ImproveStart key="improve" {...improve} />]) {
    await draw(screen);
    expect(node.querySelectorAll("h1")).toHaveLength(1);
    const heading = node.querySelector("h1")!;
    expect(heading.className).toBe("vibe-start-title");
    expect(getComputedStyle(heading).fontFamily).toBe("var(--vibe-font-serif)");
    expect(getComputedStyle(heading).fontSize).toBe("var(--vibe-font-display-s)");
    expect(getComputedStyle(heading).fontWeight).toBe("400");
  }
});
it("U2: Try and two example pill chips share one row", async () => {
  await draw(<BuildStart {...build} />);
  const row = node.querySelector('[role="group"][aria-label="Example tasks"]')!;
  expect(row?.firstElementChild?.textContent).toBe("Try");
  expect(row.querySelectorAll("button.vibe-example-chip")).toHaveLength(2);
  expect(getComputedStyle(row).display).toBe("flex");
  expect(style.textContent).toContain(".vibe-build-example-chips .vibe-example-chip { border-radius: 999px;");
});
it("U3: Build groups its textarea and examples in one card inside a borderless form", async () => {
  await draw(<BuildStart {...build} />);
  const form = getComputedStyle(node.querySelector("form")!);
  expect(form.borderTopWidth).toBe("0px");
  expect(parseFloat(form.padding)).toBe(0);
  expect(form.boxShadow).toBe("none");
  const card = node.querySelector(".vibe-build-start-card")!;
  expect(style.textContent).toContain("border: 1px solid var(--vibe-control-border); border-radius: var(--vibe-radius-lg); background: var(--vibe-surface);");
  expect(card.querySelector("textarea")).not.toBeNull();
  expect(card.querySelector('[aria-label="Example tasks"]')).not.toBeNull();
});
it("U4: quiet text aligns with its column and keeps 44px targets", async () => {
  await draw(<ImproveStart {...improve} />);
  for (const button of node.querySelectorAll(".vibe-button-quiet")) {
    const computed = getComputedStyle(button);
    expect(parseFloat(computed.paddingInline)).toBe(0);
    expect(computed.minHeight).toBe("44px");
    expect(computed.minWidth).toBe("44px");
  }
});
it("U5: quiet buttons keep the sample and footer switch underlined at rest", async () => {
  await draw(<ImproveStart {...improve} />);
  for (const button of node.querySelectorAll(".vibe-button-quiet")) expect(getComputedStyle(button).textDecoration).toBe(button.matches(".vibe-start-switch, .vibe-improve-sample") ? "underline" : "none");
  expect(css).toContain('.vibe-button-quiet:is(:hover, :focus-visible) { text-decoration: underline; }');
});
it.each(["build", "improve"] as const)("W1: %s keeps the shared Data use link underlined at rest", async door => {
  await draw(door === "build" ? <BuildStart {...build} /> : <ImproveStart {...improve} />);
  const link = node.querySelector<HTMLAnchorElement>('.vibe-data-use-notice a[href="/docs/vibe-evals/data-use"]');
  expect(link).not.toBeNull();
  expect(link!.textContent).toBe("Data use and retention (new tab)");
  expect(link!.target).toBe("_blank");
  expect(getComputedStyle(link!).textDecorationLine).toBe("underline");
});
it("U6: every associated input label has the same callout weight and size", async () => {
  await draw(<ImproveStart {...improve} />);
  for (const control of node.querySelectorAll<HTMLTextAreaElement | HTMLSelectElement>("textarea, select")) {
    expect(control.labels).toHaveLength(1);
    expect(getComputedStyle(control.labels![0]).fontWeight).toBe("600");
    expect(getComputedStyle(control.labels![0]).fontSize).toBe("var(--vibe-font-callout)");
  }
});
it("U7: disclosures replace native markers with rotating chevrons and respect reduced motion", () => {
  expect(css).toContain('summary { list-style: none; }');
  expect(css).toContain('summary::-webkit-details-marker { display: none; }');
  expect(css).toContain('summary::before { transform: rotate(135deg); }');
  expect(css).toContain('@media (prefers-reduced-motion: no-preference) { .vibe-workspace summary::before { transition: transform var(--vibe-dur-2) var(--vibe-ease-std); } }');
});
it("U9: price uses mono with a muted footnote underneath", async () => {
  await draw(<BuildStart {...build} />);
  const price = getComputedStyle(node.querySelector(".vibe-build-price")!);
  expect(price.fontFamily).toContain("--vibe-font-data");
  expect(price.fontSize).toBe("var(--vibe-font-callout)");
  expect(price.color).toBe("var(--vibe-fg)");
  const detail = getComputedStyle(node.querySelector(".vibe-build-price-detail")!);
  expect(detail.fontSize).toBe("var(--vibe-font-footnote)");
  expect(detail.color).toBe("var(--vibe-text-3)");
});
it("U9b: an unknown price is a sentence in the body font, with no footnote", async () => {
  await draw(<BuildStart {...build} price={{ kind: "unknown" }} />);
  const price = node.querySelector(".vibe-build-price")!;
  expect(price.getAttribute("data-kind")).toBe("unknown");
  expect(getComputedStyle(price).fontFamily).not.toContain("--vibe-font-data");
  expect(node.querySelector(".vibe-build-price-detail")).toBeNull();
});
it("P1: the entry shows three paths in one row, and one column on a phone", async () => {
  await draw(<EvaluationEntry busy={false} />);
  const paths = node.querySelector(".vibe-entry-paths")!;
  expect(paths.querySelectorAll(".vibe-entry-path")).toHaveLength(3);
  expect(getComputedStyle(paths).display).toBe("grid");
  expect(getComputedStyle(paths).gridTemplateColumns).toBe("repeat(3,minmax(0,1fr))");
  expect(style.textContent).toContain(".vibe-entry-paths, .vibe-entry-intro[data-returning] .vibe-entry-paths { grid-template-columns:minmax(0,1fr);");
  expect(node.querySelector(".vibe-door-steps")).toBeNull();
});
it("P1b: Continue reopens saved work while new projects are blocked", async () => {
  const onOpen = vi.fn();
  await draw(<EvaluationEntry busy navigationBlocked={false} recent={{ title: "Shop returns assistant", detail: "1 of 3 tests failed in the last run.", tone: "issue", onOpen }} />);
  expect(node.querySelector("h1")?.textContent).toBe("Welcome back.");
  expect([...node.querySelectorAll<HTMLButtonElement>('button[id^="vibe-door-"]')].every(door => door.disabled)).toBe(true);
  const resume = [...node.querySelectorAll<HTMLButtonElement>("button")].find(button => button.textContent === "Continue")!;
  expect(resume.disabled).toBe(false);
  await act(async () => resume.click());
  expect(onOpen).toHaveBeenCalledOnce();
});
it("U16: the trial follows the responsive section gap with a 32px fallback", () => {
  expect(css).toContain('article:has(> .vibe-build-result) + .vibe-prototype-trial { margin-block-start: var(--vibe-section-gap, var(--vibe-space-6)); }');
});
it("U8: formats raw provider IDs in both model selects without changing submitted values", async () => {
  await draw(<ImproveStart {...improve} />);
  expect(node.querySelector('option[value="openai/gpt-4.1-mini"]')!.textContent).toBe("GPT-4.1 mini · openai/gpt-4.1-mini");
  await draw(<ModelSelect label="Agent" value="openai/gpt-4.1-mini" models={[{ ...improve.models[0], input_nano_per_token: 0, output_nano_per_token: 0 }, { id: "anthropic/claude-sonnet", name: "anthropic/claude-sonnet", input_nano_per_token: 0, output_nano_per_token: 0 }]} onChange={noop} />);
  expect(node.querySelector("select")!.value).toBe("openai/gpt-4.1-mini");
  expect(node.querySelector("option")!.textContent).toBe("GPT-4.1 mini · OpenAI");
});
it("U15: has one notice at the trial input and shows it only when Guide is absent", async () => {
  const trial = <PrototypeTrial materialInput={<div className="vibe-task-input"><VibeDataUseNotice /></div>} inline workspace busy={false} text="" onText={noop} onSend={noop} onBack={noop} onNew={noop} thread="new" onThread={noop} history={[]} messages={[]} />;
  const screen = (guide: boolean) => <div className="vibe-project-body" data-guide-open={guide || undefined}><div className="vibe-project-main">{trial}</div>{guide && <aside><VibeDataUseNotice /></aside>}</div>;
  await draw(screen(true));
  const notices = () => [...node.querySelectorAll("p")].filter(notice => notice.textContent?.startsWith("Runs use model providers.")).filter(notice => getComputedStyle(notice).display !== "none");
  expect(notices()).toHaveLength(1);
  expect(notices()[0].closest("aside")).not.toBeNull();
  await draw(screen(false));
  expect(notices()).toHaveLength(1);
  expect(notices()[0].closest(".vibe-prototype-trial")).not.toBeNull();
});

it("U15: the trial keeps a notice beside its composer when materials are unavailable", async () => {
  await draw(<PrototypeTrial inline workspace busy={false} text="" onText={noop} onSend={noop} onBack={noop} onNew={noop} thread="new" onThread={noop} history={[]} messages={[]} />);
  const notices = [...node.querySelectorAll("p")].filter(notice => notice.textContent?.startsWith("Runs use model providers."));
  expect(notices).toHaveLength(1);
  expect(notices[0].closest(".vibe-prototype-trial")).not.toBeNull();
});

it("Results header tabs use transparent underline states and retain a single row", async () => {
  await draw(<header className="vibe-project-header"><nav className="vibe-nav"><button role="tab" className="vibe-project-tab" aria-selected="false">Agent</button><button role="tab" className="vibe-project-tab" aria-selected="true">Results</button></nav></header>);
  const [idle, selected] = [...node.querySelectorAll(".vibe-project-tab")].map(tab => getComputedStyle(tab));
  expect(idle.borderBottomWidth).toBe("2px");
  expect(idle.borderBottomColor).toBe("rgba(0, 0, 0, 0)");
  expect(idle.color).toBe("var(--vibe-muted)");
  expect(selected.borderBottomColor).toBe("var(--vibe-ink)");
  expect(selected.color).toBe("var(--vibe-fg)");
  expect(selected.background).toBe("rgba(0, 0, 0, 0)");
  expect(getComputedStyle(node.querySelector(".vibe-nav")!).flexWrap).toBe("nowrap");
});

it("Results identity and metadata use display and data fonts; header tools stay 44px", async () => {
  await draw(<header className="vibe-project-header"><div className="vibe-project-identity"><div><h1>A long project name</h1><p>v1 · Run 1: Completed</p></div></div><div className="vibe-project-tools"><button className="vibe-button vibe-project-guide">Guide<small className="vibe-guide-attention">Needs attention</small></button><button className="vibe-button">Details</button></div></header>);
  expect(getComputedStyle(node.querySelector("h1")!).fontFamily).toBe("var(--vibe-font-serif)");
  expect(getComputedStyle(node.querySelector("p")!).fontFamily).toBe("var(--vibe-font-data)");
  for (const tool of node.querySelectorAll("button")) expect(getComputedStyle(tool).height).toBe("44px");
  expect(getComputedStyle(node.querySelector(".vibe-project-guide")!).background).toBe("var(--vibe-surface-2)");
});

it("Results rows use fixed status columns, unboxed rules and mobile detail stacking", async () => {
  await draw(<details className="vibe-result-row" data-verdict="FAIL"><summary><span>×</span><span>Test message</span><span className="vibe-verdict-word vibe-verdict-issue">Issue</span><span>›</span></summary><div className="vibe-result-details"><div className="vibe-evidence-field"><p className="vibe-evidence-label">It replied</p><div className="vibe-evidence-content vibe-example-reply">Reply</div></div></div></details>);
  const summary = getComputedStyle(node.querySelector("summary")!);
  // jsdom keeps the UA summary display value; inspect the authored declaration.
  expect(style.textContent).toContain('.vibe-result-row[data-verdict] > summary {\n  display: grid;');
  expect(summary.gridTemplateColumns).toBe("24px minmax(0, 1fr) minmax(56px, max-content) 24px");
  expect(getComputedStyle(node.querySelector(".vibe-evidence-field")!).gridTemplateColumns).toBe("96px minmax(0,1fr)");
  expect(getComputedStyle(node.querySelector(".vibe-example-reply")!).borderLeftColor).toBe("var(--vibe-issue)");
  expect(style.textContent).toContain('.vibe-result-row[data-verdict] .vibe-evidence-field { grid-template-columns:minmax(0,1fr); gap:var(--vibe-space-1); }');
  expect(style.textContent).toContain('.vibe-result-row[data-verdict] .vibe-verdict-word.vibe-verdict-issue { background:var(--vibe-issue-tint); }');
});


it("Tests tab rows keep main's three-child flex layout at desktop and mobile widths", async () => {
  // Exact TestSuitePanel row markup for a non-editable test, without optional actions.
  await draw(<div className="vibe-panel"><details className="vibe-result-row">
    <summary>
      <span className="w-5 shrink-0 text-xs vibe-muted">1</span>
      <span className="vibe-row-title min-w-0 flex-1">A customer message</span>
      <ChevronRight className="vibe-chevron vibe-muted" size={16} />
    </summary>
    <div className="border-t border-[var(--vibe-border)] p-4">
      <p className="mb-1 text-xs vibe-muted">What should happen</p>
      <p>Follow the policy.</p>
      <p className="mt-2 text-sm vibe-muted">To change custom grading rules, edit the original pack and import it again.</p>
    </div>
  </details></div>);
  const summary = node.querySelector("summary")!;
  expect(summary.children).toHaveLength(3);
  expect(getComputedStyle(summary).gridTemplateColumns).toBe("none");
  expect(style.textContent).toContain(".vibe-result-row summary {\n  display: flex;");
  // jsdom does not evaluate media queries. Check every authored template, including
  // mobile overrides, to ensure none can match the real Tests summary.
  function checkTemplates(rules: CSSRuleList) {
    for (const rule of Array.from(rules)) {
      if (rule instanceof CSSStyleRule && rule.style.getPropertyValue("grid-template-columns")) {
        expect(summary.matches(rule.selectorText), rule.selectorText).toBe(false);
      } else if ("cssRules" in rule) {
        checkTemplates((rule as CSSMediaRule).cssRules);
      }
    }
  }
  checkTemplates(style.sheet!.cssRules);
});
