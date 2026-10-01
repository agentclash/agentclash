import {expect,test} from "@playwright/test";
import {writeFile} from "node:fs/promises";
const api=`http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
const description="Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund. Prepare exactly three tests.";
test.beforeEach(async({request})=>{
 test.skip(process.env.VIBE_BROWSER_TWO_DOOR!=="1","Two-door opt-in fixture");
 await request.post(`${api}/__fixture/control`,{data:{reset_calls:true}});
});
test.afterEach(async({page},info)=>{
 if(new URL(page.url()).searchParams.get("session")) await writeFile(info.outputPath("persisted-evidence.json"),JSON.stringify(await evidence(page),null,2));
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:0}});
});
async function evidence(page: import("@playwright/test").Page) {
 const id=new URL(page.url()).searchParams.get("session");
 const result=await (await page.request.get(`${api}/__fixture/evidence?session=${id}`)).json();
 return {...result,calls:result.calls || []};
}
test("clear Build goes through real worker to three results, scoped export and interactive prototype",async({page},info)=>{
 await page.setViewportSize({width:1440,height:900});
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
 await expect(page.getByRole('article',{name:'Prototype example results'})).toBeVisible();
 await expect(page.getByRole('tab',{name:'Conversation',exact:true})).toHaveAttribute('aria-selected','true');
 await expect(page.locator('[data-message-id]').first()).toContainText('Answer shop return questions');
 await expect(page.getByRole('status').filter({hasText:'All 3 checks matched your rules.'})).toBeVisible();
 await expect(page.locator('.vibe-result-row')).toHaveCount(0);
 await expect(page.getByLabel('Active evaluation')).toBeVisible();
 await page.screenshot({path:info.outputPath('results-desktop.png'),fullPage:true});
 for(const viewport of [{width:360,height:800},{width:390,height:844}]) {
  await page.setViewportSize(viewport);
  await page.getByLabel('Active evaluation').scrollIntoViewIfNeeded();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.getByLabel('Active evaluation')).toBeInViewport();
  await page.screenshot({path:info.outputPath(`results-${viewport.width}.png`),fullPage:true});
 }
 await page.setViewportSize({width:1280,height:900});
 await page.getByRole('button',{name:'Download / save',exact:true}).click();
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'Export / build it myself',exact:true}).click();await download;
 await page.getByRole('button',{name:'Try it yourself',exact:true}).click();
 await expect(page.getByText(/business systems aren’t connected/).first()).toBeVisible();
 await page.getByRole('textbox',{name:'Message your agent',exact:true}).fill('My item is unopened and I bought it 10 days ago.');
 await page.getByRole('button',{name:'Send to prototype',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('COMPLETED');
 await expect.poll(async()=>{const v=await evidence(page);return v.calls.filter((c:{role:string})=>c.role==='target').length;}).toBe(4);
 expect(errors).toEqual([]);
 const afterTrial=await evidence(page);
 expect(afterTrial.session.document.artifacts[0].blueprint).toEqual(v.session.document.artifacts[0].blueprint);
 expect(afterTrial.calls.filter((c:{role:string})=>c.role==='evaluator')).toHaveLength(v.calls.filter((c:{role:string})=>c.role==='evaluator').length);
 await page.screenshot({path:info.outputPath('interactive-prototype.png'),fullPage:true});
 await page.reload();
 await expect(page.getByRole('article',{name:'Prototype example results'})).toHaveCount(1);
 await expect(page.locator('[data-message-id]').first()).toContainText('Answer shop return questions');
 expect((await evidence(page)).calls).toHaveLength(afterTrial.calls.length);
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
 for(const viewport of [{width:360,height:800},{width:390,height:844},{width:768,height:900},{width:1440,height:900}]) {
  await page.setViewportSize(viewport);await page.goto('/vibe-evals');
  await expect(page.getByRole('button',{name:/Improve an existing agent/})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath(`entry-${viewport.width}.png`),fullPage:true});
 }
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 for(const name of ['Import a test pack','Paste agent instructions']) await expect(page.getByRole('button',{name,exact:true})).toBeVisible();
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
 await page.getByRole('button',{name:'New agent',exact:true}).click();
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 await expect(page.getByRole('heading',{name:'Improve an agent you already have.'})).toBeVisible();
 const second=new URL(page.url()).searchParams.get('session');expect(second).not.toBe(first);
 const composer=page.getByRole('textbox',{name:'Message Vibe Evals'});
 await composer.fill('My unsent support agent brief');
 await expect.poll(async()=>{const v=await (await page.request.get(`${api}/__fixture/evidence?session=${first}`)).json();return v.session.document.build?.phase;},{timeout:60000}).toBe('results');
 await expect(page.locator(`[data-evaluation-id="${second}"]`)).toHaveAttribute('aria-current','page');
 await expect(composer).toHaveValue('My unsent support agent brief');
 expect((await evidence(page)).session.document.artifacts).toHaveLength(0);
 await page.locator(`[data-evaluation-id="${first}"]`).click();
 await page.locator(`[data-evaluation-id="${second}"]`).click();
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
 await page.getByRole('button',{name:'New agent',exact:true}).click();await page.getByRole('button',{name:/Improve an existing agent/}).click();
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

test("tougher situations prepare and run one authorized batch while preserving the baseline",async({page})=>{
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('results');
 const before=await evidence(page), baseline=before.session.document.artifacts.at(-1);
 await page.locator('.vibe-build-next-actions:not([hidden]) .vibe-build-more > summary').click();
 await page.getByRole('button',{name:'Try tougher situations',exact:true}).click();
 await expect(page.getByText('Prepare 2 additional examples.',{exact:false})).toBeVisible();
 expect((await evidence(page)).session.operations).toHaveLength(2);
 await page.getByRole('button',{name:'See cost and continue',exact:true}).click();
 await page.getByRole('button',{name:'Prepare and run this batch',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.artifacts.length;}).toBe(2);
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('results');
 const after=await evidence(page), expanded=after.session.document.artifacts.at(-1);
 expect(expanded.blueprint.cases).toHaveLength(5);
 expect(expanded.blueprint.cases.slice(0,3)).toEqual(baseline.blueprint.cases);
 expect(expanded.blueprint.judges).toEqual(baseline.blueprint.judges);
 expect(expanded.agent_prompt).toBe(baseline.agent_prompt);
 expect(after.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(2);
 expect(after.session.operations.at(-1).results).toHaveLength(5);
 expect(after.session.operations.find((o:{id:string})=>o.id===before.session.operations.at(-1).id).results).toEqual(before.session.operations.at(-1).results);
 await page.reload();
 expect((await evidence(page)).session.operations).toHaveLength(4);

});

test("Build keeps the failed reply, proposed fix and authorized same-test improvement in one thread",async({page},info)=>{
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 const composer=page.getByRole('textbox',{name:'Message Vibe Evals'});
 await composer.fill(description);await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await composer.press('Enter');
 await expect.poll(async()=> (await evidence(page)).session.document.build?.phase).toBe('results');
 const original=await evidence(page), target=original.session.document.artifacts.at(-1);
 // A controlled instruction edit yields a real failed target reply through the
 // worker; no fabricated result is injected into the frontend or database.
 const patch=await page.request.patch(`${api}/v1/vibe/sessions/${original.session.id}`,{data:{revision:original.session.revision,artifact_id:target.id,agent_prompt:'Opened items are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.'}});
 expect(patch.ok(),await patch.text()).toBe(true);
 const updated=await patch.json();
 await page.goto(`/vibe-evals?session=${original.session.id}&agent=${updated.document.artifacts.at(-1).id}&view=build`);
 const proposal=page.locator('summary').filter({hasText:'Review the suggested fix'}).last();
 if(await proposal.isVisible()) await proposal.click();
 await page.getByRole('button',{name:'Apply change and check again',exact:true}).click();
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=> {const v=await evidence(page);return v.session.operations.at(-1)?.scorecard?.failed;}).toBe(1);
 const failed=await evidence(page), baseline=failed.session.operations.at(-1);
 await expect(page.getByRole('tab',{name:'Conversation',exact:true})).toHaveAttribute('aria-selected','true');
 const failedCard=page.locator(`[data-run-id="${baseline.id}"]`);
 await failedCard.locator('.vibe-build-examples summary').filter({hasText:'Issue found'}).click();

 await expect(failedCard.getByText('Opened items are eligible.',{exact:true}).first()).toBeVisible();
 await failedCard.getByRole('button',{name:'Review a fix',exact:true}).click();
 await expect.poll(async()=> (await evidence(page)).session.document.artifacts.length).toBe(3);
 const proposed=await evidence(page);
 expect(proposed.session.operations.filter((o:{kind:string})=>o.kind==='check'||o.kind==='retest')).toHaveLength(2);
 await expect(page.getByLabel('Instruction changes',{exact:true}).last()).toBeVisible();
 await expect(page.getByLabel('Instruction changes',{exact:true}).last()).toContainText('Only unopened');
 await page.getByRole('button',{name:'Review update',exact:true}).click();
 await page.getByRole('button',{name:'Apply change and check again',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=> {const v=await evidence(page);return v.session.operations.at(-1)?.scorecard?.passed;}).toBe(3);
 const final=await evidence(page), comparison=final.session.operations.at(-1);
 expect(comparison.baseline_id).toBe(baseline.id);expect(comparison.grading.hash).toBe(baseline.grading.hash);
 expect(final.session.document.artifacts.at(-1).blueprint).toEqual(target.blueprint);
 expect(final.session.operations.find((o:{id:string})=>o.id===baseline.id).results).toEqual(baseline.results);
 await expect(page.getByText('Same examples and grading · 1 improved · 0 regressed. Earlier results are kept.',{exact:true})).toBeVisible();
 await expect(page.getByRole('article',{name:'Prototype example results'})).toHaveCount(3);
 await expect(page.locator('[data-message-id]').first()).toContainText('Answer shop return questions');
 await page.screenshot({path:info.outputPath('build-same-test-improvement.png')});
});

test("sample continuation retains its prototype through harder requests, chat, real policy and export", async ({page},info)=>{
 await page.goto('/vibe-evals'); await page.getByRole('button',{name:/Build an agent/}).click();
 await expect.poll(()=>new URL(page.url()).searchParams.get('session')).toBeTruthy();
 const callsBefore=(await evidence(page)).calls.filter((c:{role:string})=>c.role==='target').length;
 const composer=page.getByRole('textbox',{name:'Message Vibe Evals',exact:true});
 async function send(text:string){
  await expect.poll(()=>new URL(page.url()).searchParams.get('session')).toBeTruthy();
  const before=(await evidence(page)).session.operations.length;
  await composer.fill(text);await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await composer.press('Enter');
  await expect.poll(async()=>{const x=(await evidence(page)).session;return x.operations.length>before && ['COMPLETED','FAILED'].includes(x.operations.at(-1)?.state)?x.operations.at(-1)?.state:'waiting';},{timeout:60000}).toBe('COMPLETED');
 }
 await send('I waste time sorting emails. Separate spam from customer messages.');

 await expect.poll(async()=>(await evidence(page)).session.document.build.phase,{timeout:60000}).toBe('results');
 let v=await evidence(page);const base=v.session.document.artifacts[0];
 expect(base.sample_basis.sample_basis).toBe('email_sorting');
 await expect(page.getByLabel('Active evaluation',{exact:true})).toContainText('Sample v1');
 await page.reload();await send('harden these tests bro');
 v=await evidence(page);const harder=v.session.document.artifacts.at(-1);
 expect(harder.id).not.toBe(base.id);expect(harder.blueprint.cases).toHaveLength(5);
 expect(harder.blueprint.cases.slice(0,3)).toEqual(base.blueprint.cases);
 expect(harder.agent_prompt).toBe(base.agent_prompt);
 expect(v.session.document.active_artifact_id).toBe(base.id);
 await expect(page.getByRole('button',{name:'Try it yourself',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Review update',exact:true}).click();
 await page.getByRole('button',{name:'Update and check',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('5 examples');
 expect((await evidence(page)).session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 await page.getByRole('button',{name:'Run 5 examples',exact:true}).click();
 await expect.poll(async()=>(await evidence(page)).session.operations.some((o:{state:string,source?:{artifact_id:string}})=>o.source?.artifact_id===harder.id && o.state==='COMPLETED')).toBe(true);
 v=await evidence(page);expect(v.session.document.active_artifact_id).toBe(harder.id);
 await send('give me vodka');
 // A typo-filled continuation must still see the working sample. Use a fresh
 // variation request through the scripted router; no new policy is introduced.
 await send('use your brain and hardnet these tests');
 v=await evidence(page);expect(v.session.document.artifacts.at(-1).sample).toBe('email_sorting');
 expect(v.session.document.active_artifact_id).toBe(harder.id);
 await send('Anything from an unknown sender is spam');
 v=await evidence(page);const actual=v.session.document.artifacts.at(-1);
 expect(actual.agent_prompt).toContain('Anything from an unknown sender is spam');
 expect(actual.agent_prompt).not.toContain('prize');expect(actual.sample).toBeFalsy();
 expect(JSON.stringify(actual.blueprint)).not.toContain('vodka');
 expect(JSON.stringify(actual.blueprint)).not.toContain('Known senders are safe');
 expect(v.session.document.active_artifact_id).toBe(harder.id);
 expect(v.session.document.build.clarifications_used).toBe(0);
 await page.locator(`[data-build-entry="prototype:${actual.id}"]`).getByRole('button',{name:'Review update',exact:true}).click();
 await page.getByRole('button',{name:'Update and check',exact:true}).click();
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=>(await evidence(page)).session.operations.some((o:{state:string,source?:{artifact_id:string}})=>o.source?.artifact_id===actual.id && o.state==='COMPLETED')).toBe(true);
 v=await evidence(page);expect(v.session.document.active_artifact_id).toBe(actual.id);
 expect(v.session.operations.at(-1).scorecard.total).toBe(3);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore+11);
 await page.getByRole('tab',{name:'Conversation',exact:true}).click();
 await page.getByRole('button',{name:'Download / save',exact:true}).click();
 const download=page.waitForEvent('download'); await page.getByRole('button',{name:'Export / build it myself',exact:true}).click();await download;
 await page.screenshot({path:info.outputPath('sample-followup-real-policy.png'),fullPage:true});
});

test("pasted instructions remain the exact target and run only after the disclosed approval", async ({page}) => {
 await page.goto('/vibe-evals');
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 await expect.poll(()=>new URL(page.url()).searchParams.get('session')).toBeTruthy();
 const callsBefore=(await evidence(page)).calls.filter((c:{role:string})=>c.role==='target').length;
 await page.getByRole('button',{name:'Paste agent instructions',exact:true}).click();
 const instructions='Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.';
 await page.getByRole('textbox',{name:'Agent instructions to test',exact:true}).fill(instructions);
 await page.getByRole('button',{name:'Prepare examples',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.artifacts.length;},{timeout:90000}).toBe(1);
 let v=await evidence(page);
 expect(v.session.document.artifacts[0].agent_prompt).toBe(instructions);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore);
 await page.getByRole('button',{name:'Run 3 tests',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 expect((await evidence(page)).calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore);
 await page.getByRole('button',{name:'Run 3 examples',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.find((o:{kind:string})=>o.kind==='check')?.state;},{timeout:90000}).toBe('COMPLETED');
 v=await evidence(page);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore+3);
 expect(v.session.operations.at(-1).results).toHaveLength(3);
 expect(v.session.document.artifacts.at(-1).agent_prompt).toBe(instructions);
 await page.reload();
 expect((await evidence(page)).session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
});
