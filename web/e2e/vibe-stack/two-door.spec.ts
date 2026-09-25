import {expect,test} from "@playwright/test";
import {writeFile} from "node:fs/promises";
const api=`http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
const description="Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund. Prepare exactly three tests.";
test.beforeEach(()=>test.skip(process.env.VIBE_BROWSER_TWO_DOOR!=="1","Two-door opt-in fixture"));
test.afterEach(async({page},info)=>{
 if(new URL(page.url()).searchParams.get("session")) await writeFile(info.outputPath("persisted-evidence.json"),JSON.stringify(await evidence(page),null,2));
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:0}});
});
async function evidence(page: import("@playwright/test").Page) {
 const id=new URL(page.url()).searchParams.get("session");
 return (await page.request.get(`${api}/__fixture/evidence?session=${id}`)).json();
}
test("clear Build goes through real worker to three results, scoped export and interactive prototype",async({page},info)=>{
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await page.goto('/vibe-evals');
 await expect(page.getByRole('button',{name:/Build an agent/})).toBeVisible();
 await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 const send=page.getByRole('button',{name:'Send message',exact:true});
 await expect(send).toHaveText(/Build and try 3 examples/);
 await expect(send).toBeEnabled();
 await send.click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;},{timeout:90000}).toBe('results');
 const v=await evidence(page);
 expect(v.session.document.build.clarifications_used).toBe(0);
 expect(v.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 expect(v.session.operations.at(-1).results).toHaveLength(3);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(3);
 await expect(page.getByRole('article',{name:'Evaluation scorecard'})).toBeVisible();
 await expect(page.getByText(/All these examples passed/)).toBeVisible();
 await expect(page.getByLabel('Active evaluation')).toBeVisible();
 await page.screenshot({path:info.outputPath('results-desktop.png'),fullPage:true});
 for(const width of [320,390]) {
  await page.setViewportSize({width,height:900});
  await page.getByLabel('Active evaluation').scrollIntoViewIfNeeded();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.getByLabel('Active evaluation')).toBeInViewport();
  await page.screenshot({path:info.outputPath(`results-${width}.png`),fullPage:true});
 }
 await page.setViewportSize({width:1280,height:900});
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'Export / build it myself',exact:true}).click();await download;
 await page.getByRole('button',{name:'Talk to this prototype'}).click();
 await expect(page.getByText(/business systems aren’t connected/).first()).toBeVisible();
 await page.getByRole('textbox',{name:'Message your agent',exact:true}).fill('My item is unopened and I bought it 10 days ago.');
 await page.getByRole('button',{name:'Send to agent',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('COMPLETED');
 await expect.poll(async()=>{const v=await evidence(page);return v.calls.filter((c:{role:string})=>c.role==='target').length;}).toBe(4);
 expect(errors).toEqual([]);
 await page.screenshot({path:info.outputPath('interactive-prototype.png'),fullPage:true});
});

test("mobile loading exposes Stop and cancelling does not restart the initial check",async({page},info)=>{
 await page.setViewportSize({width:390,height:844});
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:3000}});
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('checking');
 await expect(page.locator('.vibe-status-shine')).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:info.outputPath('loading-mobile.png'),fullPage:true});
 await page.emulateMedia({reducedMotion:'reduce'});await expect(page.locator('.vibe-status-shine')).not.toBeVisible();
 await page.getByRole('button',{name:'Stop',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('CANCELLED');
 await page.reload();
 const v=await evidence(page);expect(v.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 expect(v.session.document.artifacts).toHaveLength(1);
 await expect(page.getByText('All these examples passed.',{exact:false})).not.toBeVisible();
});
test("one question then unknown produces a labelled sample, with no invented requirements",async({page},info)=>{
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill('Build me a returns agent for Shopify');
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect(page.getByRole('button',{name:'Use a sample policy'})).toBeVisible();
 await page.reload();await page.getByRole('button',{name:'Use a sample policy'}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;},{timeout:90000}).toBe('results');
 const v=await evidence(page);expect(v.session.document.build.clarifications_used).toBe(1);
 expect(v.session.document.artifacts[0].sample).toBe('returns');
 expect(v.session.document.policies || []).toHaveLength(0);
 await expect(page.getByText(/These results check the sample/)).toBeVisible();
 await page.screenshot({path:info.outputPath('sample-results.png'),fullPage:true});
});
test("entry and context controls fit desktop and mobile",async({page},info)=>{
 for(const width of [320,390,768,1280,1440]) {
  await page.setViewportSize({width,height:900});await page.goto('/vibe-evals');
  await expect(page.getByRole('button',{name:/Test what you have/})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath(`entry-${width}.png`),fullPage:true});
 }
 await page.getByRole('button',{name:/Test what you have/}).click();
 for(const name of ['Import challenge pack','Paste conversation','Use agent instructions']) await expect(page.getByRole('button',{name,exact:true})).toBeVisible();
 await expect(page.getByText(/Live connections aren’t available/)).toBeVisible();
});

test("switching while a run completes keeps messages and draft attached to the selected evaluation",async({page})=>{
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:2000}});
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 const first=new URL(page.url()).searchParams.get('session');
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('checking');
 await page.locator('summary').filter({hasText:'New evaluation'}).click();
 await page.getByRole('button',{name:'Test what you have',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Bring what you have.'})).toBeVisible();
 const second=new URL(page.url()).searchParams.get('session');expect(second).not.toBe(first);
 const composer=page.getByRole('textbox',{name:'Message Vibe Evals'});
 await composer.fill('My unsent support agent brief');
 await expect.poll(async()=>{const v=await (await page.request.get(`${api}/__fixture/evidence?session=${first}`)).json();return v.session.document.build?.phase;},{timeout:60000}).toBe('results');
 await expect(page.getByLabel('Active evaluation')).toHaveValue(second!);
 await expect(composer).toHaveValue('My unsent support agent brief');
 expect((await evidence(page)).session.document.artifacts).toHaveLength(0);
 await page.getByLabel('Active evaluation').selectOption(first!);
 await page.getByLabel('Active evaluation').selectOption(second!);
 await expect(composer).toHaveValue('My unsent support agent brief');
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:0}});
});

test("imported tests need independent target instructions and keep identical grading through a fix",async({page},info)=>{
 // Seed a real compiled pack through Build, then import its tests in another evaluation.
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('results');
 const original=(await evidence(page)).session.document.artifacts[0].blueprint;
 await page.locator('summary').filter({hasText:'New evaluation'}).click();await page.getByRole('button',{name:'Test what you have',exact:true}).click();
 await page.locator('input[type=file]').setInputFiles({name:'returns.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(original))});
 await expect(page.getByRole('button',{name:'Add your agent',exact:true})).toBeVisible();
 let state=await evidence(page);expect(state.session.operations).toHaveLength(0);expect(state.session.document.artifacts.at(-1).blueprint).toEqual(original);
 await page.getByRole('button',{name:'Create a prototype',exact:true}).click();
 await page.getByRole('textbox',{name:'Its job and your rules',exact:true}).fill('Opened items are eligible. Unopened items within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.');
 await page.getByRole('button',{name:'Create interactive prototype',exact:true}).click();
 await page.getByRole('button',{name:'Run 3 tests',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 expect((await evidence(page)).session.operations).toHaveLength(0);
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('COMPLETED');
 state=await evidence(page);const baseline=state.session.operations.at(-1);expect(baseline.scorecard.failed).toBe(1);
 // Existing prompt-diff flow: user edits instructions, never the imported expected answers.
 const target=state.session.document.artifacts.at(-1);
 const patch=await page.request.patch(`${api}/v1/vibe/sessions/${state.session.id}`,{data:{revision:state.session.revision,artifact_id:target.id,agent_prompt:target.agent_prompt.replace('Opened items are eligible.','Only unopened items are eligible.')}});expect(patch.ok(),await patch.text()).toBe(true);
 const updated=await patch.json();
 await page.goto(`/vibe-evals?session=${state.session.id}&agent=${updated.document.artifacts.at(-1).id}&view=build`);
 const conversation=page.getByRole('tab',{name:'Conversation',exact:true});if(await conversation.count()) await conversation.click();
 const review=page.locator('summary').filter({hasText:'Review the suggested fix'});if(await review.isVisible()) await review.click();
 await page.getByRole('button',{name:'Improve and rerun',exact:true}).click();
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.length===2 && v.session.operations.at(-1)?.state==='COMPLETED';}).toBe(true);
 state=await evidence(page);const rerun=state.session.operations.at(-1);
 expect(rerun.baseline_id).toBe(baseline.id);expect(rerun.grading.hash).toBe(baseline.grading.hash);
 expect(rerun.scorecard.passed).toBe(3);expect(state.session.document.artifacts.at(-1).blueprint).toEqual(original);
 await expect(page.getByText('1 fixed · 0 new failures. Same tests and evaluator.')).toBeVisible();
 await page.screenshot({path:info.outputPath('same-tests-improvement.png'),fullPage:true});
});

test("tougher situations add bounded coverage without changing or automatically rerunning the baseline",async({page})=>{
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('results');
 const before=await evidence(page), baseline=before.session.document.artifacts.at(-1);
 await page.getByRole('button',{name:'Try tougher situations',exact:true}).click();
 await expect(page.getByText('Prepare 2 additional examples.',{exact:false})).toBeVisible();
 expect((await evidence(page)).session.operations).toHaveLength(2);
 await page.getByRole('button',{name:'Prepare these examples',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.artifacts.length;}).toBe(2);
 const after=await evidence(page), expanded=after.session.document.artifacts.at(-1);
 expect(expanded.blueprint.cases).toHaveLength(5);
 expect(expanded.blueprint.cases.slice(0,3)).toEqual(baseline.blueprint.cases);
 expect(expanded.blueprint.judges).toEqual(baseline.blueprint.judges);
 expect(expanded.agent_prompt).toBe(baseline.agent_prompt);
 expect(after.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 expect(after.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(3);
 const conversation=page.getByRole('tab',{name:'Conversation',exact:true});if(await conversation.count()) await conversation.click();
 const review=page.locator('summary').filter({hasText:'Review your tests'}).last();
 if(await review.isVisible() && !(await review.evaluate(node=>(node.parentElement as HTMLDetailsElement).open))) await review.click();
 await page.getByRole('button',{name:'Run 5 tests',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 await expect(page.getByRole('button',{name:'Run 5 examples',exact:true})).toBeVisible();
 expect((await evidence(page)).session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
});
