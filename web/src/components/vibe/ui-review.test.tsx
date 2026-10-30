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
  expect(row?.firstElementChild?.textContent).toBe("Try:");
  expect(row.querySelectorAll("button.vibe-example-chip")).toHaveLength(2);
  expect(getComputedStyle(row).display).toBe("flex");
  expect(style.textContent).toContain(".vibe-build-example-chips .vibe-example-chip { border-radius: 999px;");
});
it("U3: Build has one input boundary and no surrounding form border or padding", async () => {
  await draw(<BuildStart {...build} />);
  const form = getComputedStyle(node.querySelector("form")!);
  expect(form.borderTopWidth).toBe("0px");
  expect(parseFloat(form.padding)).toBe(0);
  expect(form.boxShadow).toBe("none");
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
it("U5: both starts use plain quiet links with a hover/focus underline", async () => {
  await draw(<ImproveStart {...improve} />);
  for (const button of node.querySelectorAll(".vibe-button-quiet")) expect(getComputedStyle(button).textDecoration).toBe("none");
  expect(css).toContain(':is(a, .vibe-button-quiet):is(:hover, :focus-visible) { text-decoration: underline; }');
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
it("U9: the free trial uses muted sans metadata", async () => {
  await draw(<BuildStart {...build} />);
  const price = getComputedStyle(node.querySelector(".vibe-build-price")!);
  expect(price.fontFamily).toContain("--font-sans");
  expect(price.fontSize).toBe("var(--vibe-font-footnote)");
  expect(price.color).toBe("var(--vibe-muted)");
});
it("P1: entry steps form a single column with single-line labels at every width", async () => {
  await draw(<EvaluationEntry busy={false} />);
  const steps = node.querySelector(".vibe-door-steps")!;
  expect(steps.children).toHaveLength(3);
  expect(getComputedStyle(steps).display).toBe("grid");
  expect(getComputedStyle(steps).gridTemplateColumns).toBe("minmax(0,1fr)");
  for (const step of steps.children) expect(getComputedStyle(step).whiteSpace).toBe("nowrap");
  expect(style.textContent).toContain('.vibe-door-steps { grid-template-columns:minmax(0,1fr); }');
});
it("U16: the trial follows the result action area by the existing 32px spacing token", () => {
  expect(css).toContain('article:has(> .vibe-build-result) + .vibe-prototype-trial { margin-block-start: var(--vibe-space-6); }');
});
it("U8: formats raw provider IDs in both model selects without changing submitted values", async () => {
  await draw(<ImproveStart {...improve} />);
  expect(node.querySelector('option[value="openai/gpt-4.1-mini"]')!.textContent).toBe("GPT-4.1 mini · OpenAI");
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
