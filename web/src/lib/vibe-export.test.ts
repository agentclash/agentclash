import { describe, expect, it, vi } from "vitest";
import { exportAgent, exportRuns } from "./vibe-export";
import type { Session, CaseResult } from "./vibe";
import type { Artifact, Models } from "./vibe";

const session = {operations: [{id: "run-a", results: [{case_key: "one"}, {case_key: "two"}]}]} as Session;

describe("saved evidence export", () => {
  it("downloads only the chosen definition with transferable reference requirements", async () => {
    const artifact = {id:"private-artifact-id", title:"Policy assistant", agent_prompt:"Answer using the reference", blueprint:{cases:[]}, scope_note:"Recommendations only", input_contract:{version:1,formats:["text"],label:"Question"}, required_capabilities:["text_generation"], reference_inputs:[{input_id:"private-input-id",content_hash:"a".repeat(64),usage:"reference"}]} as Artifact;
    const models: Models = {assistant:"assistant",target:"target",evaluator:"evaluator"};
    let downloaded: Blob | undefined;
    vi.stubGlobal("URL", {createObjectURL: vi.fn((blob: Blob) => {downloaded = blob; return "blob:definition";}),revokeObjectURL:vi.fn()});
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    try {
      exportAgent(artifact, models, [{id:"private-input-id",content_hash:"a".repeat(64),name:"Policy.txt",kind:"text",page_count:1,pages:[{number:1,text:"PRIVATE_DOCUMENT_CONTENT"}]} as import("./vibe-inputs").TaskMaterial]);
      const text = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = reject; reader.readAsText(downloaded!);
      });
      const data = JSON.parse(text);
      expect(data).toMatchObject({format:"agentclash-vibe-v2", title:artifact.title, agent_prompt:artifact.agent_prompt,evaluation:artifact.blueprint, scope_note:artifact.scope_note, input_contract:artifact.input_contract, required_capabilities:artifact.required_capabilities, references:[{key:"reference-1",content_hash:"a".repeat(64),usage:"reference",name:"Policy.txt",format:"text",page_count:1}], models});
      expect(text).not.toContain("private-artifact-id");
      expect(text).not.toContain("private-input-id");
      expect(text).not.toContain("PRIVATE_DOCUMENT_CONTENT");
    } finally {click.mockRestore(); vi.unstubAllGlobals();}
  });
  it("retrieves the original bodies without modifying the redacted snapshot", async () => {
    const load = vi.fn(async (_: string, key: string) => ({case_key: key, input: "actual input", output: "actual reply"}) as CaseResult);
    const runs = await exportRuns(session, load);
    expect(load).toHaveBeenCalledWith("run-a", "one");
    expect(load).toHaveBeenCalledWith("run-a", "two");
    expect(runs[0].results.map(r => r.output)).toEqual(["actual reply", "actual reply"]);
    expect(session.operations[0].results[0].output).toBeUndefined();
  });

  it("does not present an incomplete archive as a successful full export", async () => {
    await expect(exportRuns(session, async () => {throw new Error("Unavailable");})).rejects.toThrow("Unavailable");
  });
});
