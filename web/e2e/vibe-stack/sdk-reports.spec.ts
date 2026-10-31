import { test, expect, type Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { deferInstructionsAndKeepTests } from "./keep-tests";
const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT || "55441"}`;
const workspace = "a8000000-0000-4000-8000-000000000003";
const blueprint = {
 slug:"sdk-returns",name:"SDK retained reply tests",description:"Review a retained reply",difficulty:"easy",instructions:"{{question}}",
 cases:[{key:"case-1",payload:{question:"I bought this unopened item 10 days ago. Can I return it?"},expectations:[{key:"expected_behavior",kind:"text",value:"Confirm eligibility under the 30-day unopened-item policy."}]}],
 validators:[{key:"has_answer",type:"regex_match",target:"final_output",expected_from:"literal:.+"}],
 judges:[{key:"behavior",mode:"assertion",assertion:"The response meets this case's expected_behavior and the following shared rules:\n\nNever claim to process a refund.",context_from:["case.expectations.expected_behavior"]}],
 dimensions:[{key:"output_present",source:"validators",validators:["has_answer"]},{key:"behavior_correctness",source:"llm_judge",judge_key:"behavior"}],
};
async function evidence(page:Page,id:string){const value=await (await page.request.get(`${api}/__fixture/evidence?session=${id}`)).json();return {...value,calls:value.calls || []}}
test("mounted SDK import keeps exact bytes, source URL, reviews, separate regrade and reviewed promotion",async({page})=>{
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await page.goto("/auth/login?returnTo=%2Fvibe-evals");await page.getByRole("button",{name:"Continue with AgentClash"}).click();
 await expect(page.getByRole("button", { name: /Build an agent/ })).toBeEnabled(); await expect(page.getByLabel("Workspace for new projects")).toHaveCount(0);
 await page.getByRole("button",{name:/Improve an existing agent/}).click();
 await page.getByRole("radio",{name:"Test pack",exact:true}).click();
 const importTests=page.getByRole("button",{name:"Choose a file",exact:true});await expect(importTests).toBeEnabled();
 const chooseTests=page.waitForEvent("filechooser");await importTests.click();await (await chooseTests).setFiles({name:"sdk-tests.json",mimeType:"application/json",buffer:Buffer.from(JSON.stringify(blueprint))});
 await expect(page.getByRole("tab",{name:"Tests",exact:true})).toBeVisible();
 const id=new URL(page.url()).searchParams.get("session")!;
 await expect.poll(async()=>((await evidence(page,id)).session.document.artifacts || []).length).toBe(1);
 const initial=await evidence(page,id);const artifact=initial.session.document.artifacts[0];expect(initial.session.operations).toHaveLength(0);expect(initial.session.workspace_id).toBe(workspace);
 await page.getByRole("tab",{name:"Results",exact:true}).click();
 await expect(page.getByLabel("SDK report history",{exact:true})).toHaveCount(0);
 await expect(page.getByLabel("Application run history",{exact:true})).toHaveCount(0);
 await expect(page.locator("summary").filter({hasText:/^Import SDK report$/})).toBeVisible();
 await page.locator("summary").filter({hasText:/^Import SDK report$/}).click();
 const source=" \r\n"+JSON.stringify({schema_version:1,report_id:"python-local-original",generated_at:"2026-10-26T12:00:00Z",runner:{name:"agentclash-evals",version:"1"},summary:{total:1,passed:0,failed:1,skipped:0,errored:0},cases:[{case:{case_id:"local arbitrary id",name:"Retained answer",expected:"Confirm eligibility without claiming that a refund was processed."},status:"failed",metrics:[{key:"uploaded-rule",name:"Uploaded rule",passed:false,score:0.2,reason:"Original uploaded judgment"}],agent_result:{input:"Can I return my unopened item bought 10 days ago?",output:"Your unopened item bought 10 days ago is eligible under the 30-day policy. I cannot process a refund."}}],exit_code:1},null,2)+"\t\n";
 await page.getByRole("region",{name:"Import SDK report",exact:true}).locator('input[type="file"]').setInputFiles({name:"original-sdk.json",mimeType:"application/json",buffer:Buffer.from(source)});
 const mapping=page.getByLabel("Test for local arbitrary id — Retained answer");await expect(mapping.locator("option")).toHaveCount(2);await mapping.selectOption({index:1});
 const imported=page.waitForResponse(r=>r.url().endsWith(`/sessions/${id}/reports`)&&r.request().method()==="POST");await page.getByRole("button",{name:"Import historical report",exact:true}).click();
 const response=await imported;expect(response.ok(),await response.text()).toBe(true);const report=await response.json();
 await expect(page.getByRole("article",{name:"Imported report detail"})).toBeVisible();
 expect(new URL(page.url()).searchParams.get("run")).toBe(`sdk_report:${report.id}`);expect(new URL(page.url()).searchParams.get("agent")).toBe(artifact.id);
 expect((await evidence(page,id)).session.operations).toHaveLength(0);
 const sourceURL=page.url();await page.reload();await expect(page.getByRole("article",{name:"Imported report detail"})).toBeVisible();
 const originalDownload=page.waitForEvent("download");await page.getByRole("button",{name:"Download exact original",exact:true}).click();const downloaded=await originalDownload;expect(await readFile((await downloaded.path())!,"utf8")).toBe(source);
 await page.getByLabel("SDK detail source test").selectOption("local arbitrary id");await page.getByLabel("SDK grade review reason").fill("The uploaded grade did not account for the explicit processing limitation.");await page.getByRole("button",{name:"Record grade review",exact:true}).click();await expect(page.getByRole("region",{name:"SDK grade reviews"})).toContainText("processing limitation");
 await page.getByRole("button",{name:"Export report with tests",exact:true}).click();await expect(page.getByRole("dialog")).toContainText("SDK report");
 await page.getByRole("button",{name:"Reconcile and prepare selected evidence again",exact:true}).click();
 const exportDownload=page.waitForEvent("download");await page.getByRole("button",{name:"Download exact export",exact:true}).click();const archive=JSON.parse(await readFile((await (await exportDownload).path())!,"utf8"));
 expect(archive.sdk_reports[0].binding.result_ref).toEqual({kind:"sdk_report",id:report.id});
 const retained=archive.evidence.find((item:{reference:{kind:string}})=>item.reference.kind==="sdk_report");expect(Buffer.from(retained.value.content,"base64").toString("utf8")).toBe(source);
 await page.getByRole("dialog").getByRole("button",{name:"Close",exact:true}).last().click();
 await page.getByRole("tab",{name:"Tests",exact:true}).click();
 await deferInstructionsAndKeepTests(page);
 await expect(page.getByRole("dialog")).toContainText("No baseline selected");
 await page.getByRole("dialog").getByRole("button",{name:/Keep these tests|Save and iterate/}).click();
 const advanced=page.getByRole("link",{name:"Open in full pack builder"});await expect(advanced).toBeVisible();
 const returnPath=new URL((await advanced.getAttribute("href"))!,page.url()).searchParams.get("return_to")!;
 expect(new URL(returnPath,page.url()).searchParams.get("run")).toBe(`sdk_report:${report.id}`);
 expect(new URL(returnPath,page.url()).searchParams.get("agent")).toBe(artifact.id);
 await page.getByRole("dialog").getByRole("button",{name:"Close",exact:true}).last().click();
 await page.getByRole("tab",{name:"Results",exact:true}).click();
 const callsBefore=(await evidence(page,id)).calls.length;
 await page.getByRole("button",{name:"Estimate hosted regrade",exact:true}).click();await expect(page.getByRole("button",{name:"Confirm separate hosted regrade",exact:true})).toBeVisible();expect((await evidence(page,id)).session.operations).toHaveLength(0);
 await page.getByRole("button",{name:"Confirm separate hosted regrade",exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page,id);return v.session.operations.at(-1)?.state;}).toBe("COMPLETED");
 const graded=await evidence(page,id);expect(graded.session.operations).toHaveLength(1);expect(graded.session.operations[0].evidence_metadata.source_result_ref).toEqual({kind:"sdk_report",id:report.id});expect(graded.session.operations[0].evidence_metadata.origin).toBe("regraded");expect(graded.calls.slice(callsBefore)).toHaveLength(1);expect(graded.calls.slice(callsBefore).every((c:{role:string})=>c.role==="judge")).toBe(true);
 await expect.poll(()=>new URL(page.url()).searchParams.get("run")).toBe(graded.session.operations[0].id);
 await page.getByLabel("SDK report history",{exact:true}).locator("summary").click();
 await page.getByRole("button",{name:"Open report python-local-original",exact:true}).click();await expect(page).toHaveURL(sourceURL);
 await page.getByLabel("SDK detail source test").selectOption("local arbitrary id");const promote=page.getByRole("button",{name:"Create version with this expectation",exact:true});await expect(promote).toBeDisabled();
 await page.getByRole("checkbox",{name:"I reviewed this expectation for the selected test."}).check();await promote.click();
 await expect(page.getByRole("tab",{name:"Tests",exact:true})).toHaveAttribute("aria-selected","true");
 const promoted=await evidence(page,id);expect(promoted.session.document.artifacts).toHaveLength(2);expect(promoted.session.document.artifacts[0].blueprint).toEqual(artifact.blueprint);expect(promoted.session.operations).toHaveLength(1);
 await page.goBack();await expect(page.getByRole("article",{name:"Imported report detail"})).toBeVisible();expect(new URL(page.url()).searchParams.get("agent")).toBe(artifact.id);
 const afterDownload=page.waitForEvent("download");await page.getByRole("button",{name:"Download exact original",exact:true}).click();expect(await readFile((await (await afterDownload).path())!,"utf8")).toBe(source);
 // From 1280px the Guide panel is open by default; close it while the width is stable, so a resize below
 // 1280px never races its removal. The closed choice persists across the resizes.
 const closeGuide=page.getByRole("button",{name:"Close Guide",exact:true});if(await closeGuide.isVisible())await closeGuide.click();
 for(const width of [320,390,768,1440]){
  await page.setViewportSize({width,height:900});
  if(await closeGuide.isVisible())await closeGuide.click();
  await page.getByRole("article",{name:"Imported report detail"}).scrollIntoViewIfNeeded();
  await expect(page.getByRole("article",{name:"Imported report detail"})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  expect(await page.locator(".vibe-project-main").evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);
 }
 expect(errors).toEqual([]);
});
