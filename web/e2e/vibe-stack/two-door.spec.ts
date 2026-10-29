import {expect,test,type Page} from "@playwright/test";
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
async function openGuide(page: Page) {
 const button=page.getByRole('button',{name:'Guide',exact:true});
 if(await button.getAttribute('aria-pressed')!=='true') await button.click();
}
async function openView(page: Page, name: 'Agent'|'Tests'|'Results') {
 const sheet=page.getByRole('dialog').filter({has:page.getByRole('button',{name:'Close Guide',exact:true})});
 if(await sheet.isVisible()) await sheet.getByRole('button',{name:'Close Guide',exact:true}).click();
 await page.getByRole('tab',{name,exact:true}).click();
}
// Occasional result actions share one "More actions" disclosure below the scorecard.
async function openMoreActions(page: Page) {
 const more=page.locator('details.vibe-results-more');
 await expect(more.locator(':scope > summary')).toBeVisible();
 if(!(await more.evaluate(el=>(el as HTMLDetailsElement).open))) await more.locator(':scope > summary').click();
}
async function reviewLatest(page: Page) {
 const latest=(await evidence(page)).session.document.artifacts.at(-1).id;
 if(new URL(page.url()).searchParams.get('agent')!==latest) await page.getByRole('button',{name:'Review latest version',exact:true}).click();
 await expect.poll(()=>new URL(page.url()).searchParams.get('agent')).toBe(latest);
}
test("clear Build goes through real worker to three results, scoped export and interactive prototype",async({page},info)=>{
 await page.setViewportSize({width:1440,height:900});
 const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
 await page.goto('/vibe-evals');
 await expect(page.getByText('No projects yet.',{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Retry projects',exact:true})).toHaveCount(0);
 await expect(page.getByText('Couldn’t load your projects.',{exact:false})).toHaveCount(0);
 await expect(page.getByRole('button',{name:/Build an agent/})).toBeVisible();
 await page.getByRole('button',{name:/Build an agent/}).click();
 await expect(page.locator('[data-evaluation-id][aria-current="page"]')).toBeVisible();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 const send=page.getByRole('button',{name:'Send message',exact:true});
 await expect(send).toHaveText(/Build and run 3 tests/);
 await expect(send).toBeEnabled();
 await send.click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;},{timeout:90000}).toBe('results');
 const v=await evidence(page);
 expect(v.session.document.build.clarifications_used).toBe(0);
 expect(v.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 expect(v.session.operations.at(-1).results).toHaveLength(3);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(3);
 await expect(page.getByRole('tab',{name:'Agent',exact:true})).toHaveAttribute('aria-selected','true');
 await openGuide(page);
 await expect(page.locator('[data-message-id]').first()).toContainText('Answer shop return questions');
 await openView(page,'Results');
 await expect(page.getByRole('article',{name:'Run results'})).toContainText('3 passed · 0 failed');
 await expect(page.getByLabel('Result history').locator('option:not([disabled])')).toHaveCount(1);
 await expect(page.locator('.vibe-project-header')).toContainText('Run 1: Completed');
 await page.screenshot({path:info.outputPath('results-desktop.png'),fullPage:true});
 for(const viewport of [{width:360,height:800},{width:390,height:844}]) {
  await page.setViewportSize(viewport);
  await page.locator('.vibe-project-header').scrollIntoViewIfNeeded();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.locator('.vibe-project-header')).toBeInViewport();
  await page.screenshot({path:info.outputPath(`results-${viewport.width}.png`),fullPage:true});
 }
 await page.setViewportSize({width:1280,height:900});
 await openMoreActions(page);
 await page.getByRole('button',{name:'Download / save',exact:true}).click();
 const download=page.waitForEvent('download');await page.getByRole('button',{name:'Export / build it myself',exact:true}).click();await page.getByRole('button',{name:'Download exact export',exact:true}).click();await download;await page.getByRole('button',{name:'Close',exact:true}).last().click();
 await page.getByRole('button',{name:'Try it yourself',exact:true}).click();
 await expect(page.getByRole('region',{name:'Version scope'})).toContainText(/business systems (?:are not|aren’t) connected/);
 await page.getByRole('textbox',{name:'Message your agent',exact:true}).fill('My item is unopened and I bought it 10 days ago.');
 await page.getByRole('button',{name:'Send to agent',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('COMPLETED');
 await expect.poll(async()=>{const v=await evidence(page);return v.calls.filter((c:{role:string})=>c.role==='target').length;}).toBe(4);
 expect(errors).toEqual([]);
 const afterTrial=await evidence(page);
 expect(afterTrial.session.document.artifacts[0].blueprint).toEqual(v.session.document.artifacts[0].blueprint);
 expect(afterTrial.calls.filter((c:{role:string})=>c.role==='evaluator')).toHaveLength(v.calls.filter((c:{role:string})=>c.role==='evaluator').length);
 await page.screenshot({path:info.outputPath('interactive-prototype.png'),fullPage:true});
 await page.reload();
 await openGuide(page);
 await expect(page.locator('[data-message-id]').first()).toContainText('Answer shop return questions');
 await openView(page,'Results');
 await expect(page.getByRole('article',{name:'Run results'})).toContainText('3 passed · 0 failed');
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
 await openGuide(page);
 await expect(page.locator('.vibe-status-shine')).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:info.outputPath('loading-mobile.png'),fullPage:true});
 await page.emulateMedia({reducedMotion:'reduce'});await expect(page.locator('.vibe-status-shine')).not.toBeVisible();
 await page.getByRole('button',{name:'Stop',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('CANCELLED');
 await page.reload();
 const v=await evidence(page);expect(v.session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 expect(v.session.document.artifacts).toHaveLength(1);
 await expect(page.getByText('All these tests passed.',{exact:false})).not.toBeVisible();
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
 await openView(page,'Agent');
 await expect(page.getByRole('region',{name:'Version scope'})).toContainText('Your real policy is still unspecified.');
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
 for(const name of ['Import a test pack','Try my saved instructions']) await expect(page.getByRole('button',{name,exact:true})).toBeVisible();
 await expect(page.getByText('Connect a live app from Details',{exact:false})).toBeVisible();
});

test("switching while a run completes keeps messages and draft attached to the selected evaluation",async({page})=>{
 await page.request.post(`${api}/__fixture/control`,{data:{target_delay_ms:2000}});
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 const first=new URL(page.url()).searchParams.get('session');
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('checking');
 await page.getByRole('button',{name:'New project',exact:true}).click();
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
 await page.getByRole('button',{name:'New project',exact:true}).click();await page.getByRole('button',{name:/Improve an existing agent/}).click();
 await page.locator('input[type=file]').setInputFiles({name:'returns.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(original))});
 await openView(page,'Tests');
 await expect(page.getByRole('button',{name:'Add your agent',exact:true})).toBeVisible();
 let state=await evidence(page);expect(state.session.operations).toHaveLength(0);expect(state.session.document.artifacts.at(-1).blueprint).toEqual(original);
 await page.getByRole('button',{name:'Create an agent',exact:true}).click();
 await page.getByRole('textbox',{name:'Agent instructions',exact:true}).fill('Opened items are eligible. Unopened items within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.');
 await page.getByRole('button',{name:'Use these instructions',exact:true}).click();
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Run 3 tests',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 expect((await evidence(page)).session.operations).toHaveLength(0);
 await expect(page.getByRole('dialog')).toContainText('3 tests ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.at(-1)?.state;}).toBe('COMPLETED');
 state=await evidence(page);const baseline=state.session.operations.at(-1);expect(baseline.scorecard.failed).toBe(1);
 // Existing prompt-diff flow: user edits instructions, never the imported expected answers.
 const target=state.session.document.artifacts.at(-1);
 const patch=await page.request.patch(`${api}/v1/vibe/sessions/${state.session.id}`,{data:{revision:state.session.revision,artifact_id:target.id,agent_prompt:target.agent_prompt.replace('Opened items are eligible.','Only unopened items are eligible.')}});expect(patch.ok(),await patch.text()).toBe(true);
 const updated=await patch.json();
 await page.goto(`/vibe-evals?session=${state.session.id}&agent=${updated.document.artifacts.at(-1).id}&view=build`);
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Improve and rerun',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('3 tests ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.length===2 && v.session.operations.at(-1)?.state==='COMPLETED';}).toBe(true);
 state=await evidence(page);const rerun=state.session.operations.at(-1);
 expect(rerun.baseline_id).toBe(baseline.id);expect(rerun.grading.hash).toBe(baseline.grading.hash);
 expect(rerun.scorecard.passed).toBe(3);expect(state.session.document.artifacts.at(-1).blueprint).toEqual(original);
 await openView(page,'Results');
 await expect(page.getByText('1 fixed · 0 new failures. Same tests and evaluator.')).toBeVisible();
 await page.screenshot({path:info.outputPath('same-tests-improvement.png'),fullPage:true});
});

test("tougher situations prepare and run one authorized batch while preserving the baseline",async({page})=>{
 await page.goto('/vibe-evals');await page.getByRole('button',{name:/Build an agent/}).click();
 await page.getByRole('textbox',{name:'Message Vibe Evals'}).fill(description);
 await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await page.getByRole('textbox',{name:'Message Vibe Evals'}).press('Enter');
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.build?.phase;}).toBe('results');
 const before=await evidence(page), baseline=before.session.document.artifacts.at(-1);
 await openView(page,'Results');
 await openMoreActions(page);
 await page.locator('.vibe-build-next-actions:not([hidden]) .vibe-build-more > summary').click();
 await page.getByRole('button',{name:'Add harder tests',exact:true}).click();
 await expect(page.getByText('Prepare 2 more tests.',{exact:false})).toBeVisible();
 expect((await evidence(page)).session.operations).toHaveLength(2);
 await page.getByRole('button',{name:'See cost and continue',exact:true}).click();
 await page.getByRole('button',{name:'Start run',exact:true}).click();
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

test("Build keeps the failed reply, proposed fix and authorized same-test improvement in project history",async({page},info)=>{
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
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Apply change and rerun',exact:true}).click();
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 // Capture the immutable baseline only after the last case and operation finish.
 await expect.poll(async()=> {const o=(await evidence(page)).session.operations.at(-1);return o?.state==='COMPLETED'?o.scorecard?.failed:null;}).toBe(1);
 const failed=await evidence(page), baseline=failed.session.operations.at(-1);
 await openView(page,'Results');
 const failedCard=page.getByRole('article',{name:'Run results'});
 await expect(failedCard.getByText('Opened items are eligible.',{exact:true}).first()).toBeVisible();
 await failedCard.getByRole('button',{name:'Help me fix this',exact:true}).click();
 await expect.poll(async()=> (await evidence(page)).session.document.artifacts.length).toBe(3);
 const proposed=await evidence(page);
 expect(proposed.session.operations.filter((o:{kind:string})=>o.kind==='check'||o.kind==='retest')).toHaveLength(2);
 await reviewLatest(page);
 const changes=page.getByLabel('Instruction changes',{exact:true}).last();
 const changeSummary=page.locator('summary').filter({hasText:'Review the instruction change'});
 if(!(await changeSummary.evaluate(node=>(node.parentElement as HTMLDetailsElement).open))) await changeSummary.click();
 await expect(changes).toBeVisible();
 await expect(page.getByLabel('Instruction changes',{exact:true}).last()).toContainText('Only unopened');
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Apply change and rerun',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 await expect(page.getByRole('dialog')).toContainText('3 tests ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=> {const o=(await evidence(page)).session.operations.at(-1);return o?.state==='COMPLETED'?o.scorecard?.passed:null;}).toBe(3);
 const final=await evidence(page), comparison=final.session.operations.at(-1);
 expect(comparison.baseline_id).toBe(baseline.id);expect(comparison.grading.hash).toBe(baseline.grading.hash);
 expect(final.session.document.artifacts.at(-1).blueprint).toEqual(target.blueprint);
 expect(final.session.operations.find((o:{id:string})=>o.id===baseline.id).results).toEqual(baseline.results);
 await openView(page,'Results');
 await expect(page.getByRole('article',{name:'Run results'})).toContainText(/1 (?:fixed · 0 new failures|previously failing test now passes · 0 new failures)/);
 await expect(page.getByRole('article',{name:'Run results'})).toContainText(/Same (?:tests|expectations) and evaluator/);
 const history=page.getByLabel('Result history');
 await expect(history.locator('option:not([disabled])')).toHaveCount(3);
 await history.selectOption(baseline.id);
 await expect(page.getByRole('article',{name:'Run results'})).toContainText('2 passed · 1 failed');
 await history.selectOption(comparison.id);
 await expect(page.getByRole('article',{name:'Run results'})).toContainText('3 passed · 0 failed');
 await openGuide(page);
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
  if((await evidence(page)).session.document.artifacts.length) await openGuide(page);
  await composer.fill(text);await expect(page.getByRole('button',{name:'Send message',exact:true})).toBeEnabled();await composer.press('Enter');
  await expect.poll(async()=>{const x=(await evidence(page)).session;return x.operations.length>before && ['COMPLETED','FAILED'].includes(x.operations.at(-1)?.state)?x.operations.at(-1)?.state:'waiting';},{timeout:60000}).toBe('COMPLETED');
 }
 await send('I waste time sorting emails. Separate spam from customer messages.');

 await expect.poll(async()=>(await evidence(page)).session.document.build.phase,{timeout:60000}).toBe('results');
 let v=await evidence(page);const base=v.session.document.artifacts[0];
 expect(base.sample_basis.sample_basis).toBe('email_sorting');
 await expect(page.getByRole('region',{name:'Version scope'})).toContainText('Sample demonstration');
 await page.reload();await send('harden these tests bro');
 v=await evidence(page);const harder=v.session.document.artifacts.at(-1);
 expect(harder.id).not.toBe(base.id);expect(harder.blueprint.cases).toHaveLength(5);
 expect(harder.blueprint.cases.slice(0,3)).toEqual(base.blueprint.cases);
 expect(harder.agent_prompt).toBe(base.agent_prompt);
 expect(v.session.document.active_artifact_id).toBe(base.id);
 await expect(page.getByRole('textbox',{name:'Message your agent',exact:true})).toBeVisible();
 await reviewLatest(page);
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Update and run',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('5 tests');
 expect((await evidence(page)).session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
 await expect(page.getByRole('dialog')).toContainText('5 tests ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
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
 await reviewLatest(page);
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Update and run',exact:true}).click();
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>(await evidence(page)).session.operations.some((o:{state:string,source?:{artifact_id:string}})=>o.source?.artifact_id===actual.id && o.state==='COMPLETED')).toBe(true);
 v=await evidence(page);expect(v.session.document.active_artifact_id).toBe(actual.id);
 expect(v.session.operations.at(-1).scorecard.total).toBe(3);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore+11);
 await openView(page,'Results');
 await openMoreActions(page);
 await page.getByRole('button',{name:'Download / save',exact:true}).click();
 const download=page.waitForEvent('download'); await page.getByRole('button',{name:'Export / build it myself',exact:true}).click();await page.getByRole('button',{name:'Download exact export',exact:true}).click();await download;await page.getByRole('button',{name:'Close',exact:true}).last().click();
 await page.screenshot({path:info.outputPath('sample-followup-real-policy.png'),fullPage:true});
});

test("pasted instructions remain the exact target and run only after the disclosed approval", async ({page}) => {
 await page.goto('/vibe-evals');
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 await expect.poll(()=>new URL(page.url()).searchParams.get('session')).toBeTruthy();
 const callsBefore=(await evidence(page)).calls.filter((c:{role:string})=>c.role==='target').length;
 await page.getByRole('button',{name:'Try my saved instructions',exact:true}).click();
 const instructions='Answer shop return questions. Only unopened items bought within 30 days are eligible. Ask only for missing purchase age or item condition. Never claim to process a refund.';
 await page.getByRole('textbox',{name:'Agent instructions to test',exact:true}).fill(instructions);
 await page.getByRole('button',{name:'Prepare tests',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.document.artifacts.length;},{timeout:90000}).toBe(1);
 let v=await evidence(page);
 expect(v.session.document.artifacts[0].agent_prompt).toBe(instructions);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore);
 await openView(page,'Tests');
 await page.getByRole('button',{name:'Run 3 tests',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('up to');
 expect((await evidence(page)).calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore);
 await expect(page.getByRole('dialog')).toContainText('3 tests ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>{const v=await evidence(page);return v.session.operations.find((o:{kind:string})=>o.kind==='check')?.state;},{timeout:90000}).toBe('COMPLETED');
 v=await evidence(page);
 expect(v.calls.filter((c:{role:string})=>c.role==='target')).toHaveLength(callsBefore+3);
 expect(v.session.operations.at(-1).results).toHaveLength(3);
 expect(v.session.document.artifacts.at(-1).agent_prompt).toBe(instructions);
 await page.reload();
 expect((await evidence(page)).session.operations.filter((o:{kind:string})=>o.kind==='check')).toHaveLength(1);
});

test("recorded replies intake uses an explicit quote and evaluator-only execution",async({page},info)=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.setViewportSize({width:390,height:844});
 await page.goto('/vibe-evals');
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 await page.getByRole('button',{name:'Test replies from my app',exact:true}).click();
 await expect(page.getByRole('button',{name:'Review recorded replies',exact:true})).toBeVisible();
 await page.getByLabel('Recorded replies',{exact:true}).fill('Customer: I opened it.\nAgent: I can refund that.');
 await page.getByLabel('Expected behavior',{exact:true}).fill('Opened purchases are not eligible for refunds.');
 await page.getByRole('button',{name:'Review recorded replies',exact:true}).click();
 await expect(page.getByRole('region',{name:'Recorded reply tests'})).toBeVisible();
 const saved=await evidence(page);expect(saved.calls).toHaveLength(0);expect(saved.session.document.artifacts).toHaveLength(1);
 await page.getByRole('button',{name:'Run recorded tests',exact:true}).click();
 await expect(page.getByRole('button',{name:'Start run',exact:true})).toBeEnabled();
 expect((await evidence(page)).calls).toHaveLength(0);
 await expect(page.getByRole('dialog')).toContainText('1 test ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>{const e=await evidence(page);return e.session.operations.at(-1)?.state;},{timeout:90000}).toBe('COMPLETED');
 const result=await evidence(page);expect(result.calls.map((c:{role:string})=>c.role)).toEqual(['judge']);
 expect(result.session.operations.at(-1).evidence_metadata.origin).toBe('recorded');
 await page.getByRole('tab',{name:'Results',exact:true}).click();
 await expect(page.getByRole('article',{name:'Run results'})).toBeVisible();
 await openMoreActions(page);
 await page.getByRole('button',{name:'Review a grade',exact:true}).click();
 await page.getByLabel('Grade review reason',{exact:true}).fill('The recorded reply needs a second human review.');
 await expect(page.getByRole('button',{name:'Save grade review',exact:true})).toBeEnabled();
 await page.getByRole('button',{name:'Save grade review',exact:true}).click();
 await expect(page.getByText('The recorded reply needs a second human review.',{exact:true})).toBeVisible();
 for(const width of [320,390,600,601,768,1023,1024,1279,1280,1440]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)}
 await page.screenshot({path:info.outputPath('recorded-results.png'),fullPage:true});
 await page.reload();await expect(page.getByRole('article',{name:'Run results'})).toBeVisible();
 await openMoreActions(page);
 await expect(page.getByText('The recorded reply needs a second human review.',{exact:true})).toBeVisible();
 expect((await evidence(page)).calls).toHaveLength(1);
 // Actual CreateAgent projects retain the testJourney headline; new intake is
 // available independently of that headline and never quotes the old source.
 const baseline=result.session.operations.at(-1);
 await page.getByRole('button',{name:'Add new recorded replies',exact:true}).click();
 await expect(page.getByLabel('Recorded replies',{exact:true})).toHaveValue('');
 await expect(page.getByLabel('Expected behavior',{exact:true})).toHaveValue('Opened purchases are not eligible for refunds.');
 await page.getByLabel('Recorded replies',{exact:true}).fill('Customer: I opened it.\nAgent: I can refund that now.');
 await page.getByRole('button',{name:'Review recorded replies',exact:true}).click();
 await expect(page.getByRole('region',{name:'Recorded reply tests'})).toBeVisible();
 const updated=await evidence(page);expect(updated.calls).toHaveLength(1);
 expect(updated.session.document.artifacts).toHaveLength(2);
 const artifact=updated.session.document.artifacts.at(-1);
 expect(artifact.id).not.toBe(baseline.source.artifact_id);
 expect(artifact.conversation_evaluation.evidence_set_id).not.toBe(baseline.source.evidence_set_id);
 await page.reload();await expect(page.getByRole('region',{name:'Recorded reply tests'})).toBeVisible();
 const quoted=page.waitForRequest(request=>request.url().endsWith('/run-quote')&&request.method()==='POST');
 await page.getByRole('button',{name:'Run recorded tests',exact:true}).click();
 expect((await quoted).postDataJSON()).toMatchObject({kind:'retest',baseline_id:baseline.id,artifact_id:artifact.id,evidence_set_id:artifact.conversation_evaluation.evidence_set_id});
 await expect(page.getByRole('dialog')).toContainText('1 test ·');
 await expect(page.getByRole('button',{name:'Start run',exact:true})).toBeEnabled();
 expect((await evidence(page)).calls).toHaveLength(1);
 await expect(page.getByRole('dialog')).toContainText('1 test ·');
 await page.getByRole('button',{name:'Start run',exact:true}).click();
 await expect.poll(async()=>{const e=await evidence(page);return e.session.operations.find((op:{source?:{artifact_id?:string}})=>op.source?.artifact_id===artifact.id)?.state;},{timeout:90000}).toBe('COMPLETED');
 const final=await evidence(page);expect(final.calls.map((c:{role:string})=>c.role)).toEqual(['judge','judge']);
 expect(final.session.operations.at(-1)).toMatchObject({baseline_id:baseline.id,source:{artifact_id:artifact.id,evidence_set_id:artifact.conversation_evaluation.evidence_set_id,comparison:'updated_replies'}});
 expect(final.session.operations[0].source).toEqual(baseline.source);
 await page.getByRole('tab',{name:'Results',exact:true}).click();
 // A complete run has no "Reload saved evidence"; reload the page to read the saved run.
 await page.reload();
 await expect(page.locator('.vibe-project-header')).toContainText('Run 2: Completed');
 await page.getByRole('button',{name:'Compare saved replies',exact:true}).click();
 await expect(page.getByRole('region',{name:'Before',exact:true})).toContainText('I can refund that.');
 await expect(page.getByRole('region',{name:'After',exact:true})).toContainText('I can refund that now.');
 for(const width of [390,1280]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)}
 await page.screenshot({path:info.outputPath('recorded-updated-results.png'),fullPage:true});
 expect(errors).toEqual([]);
});

test("library original bytes round-trip through browser upload and selected export without inference",async({page},info)=>{
 await page.setViewportSize({width:1280,height:900});await page.goto('/vibe-evals');
 await page.getByRole('button',{name:/Improve an existing agent/}).click();
 // Unknown canonical extension is retained exactly and remains explicitly uneditable.
 const original=Buffer.concat([Buffer.from([0xef,0xbb,0xbf]),Buffer.from('{\r\n  "pack": {"name":"Escaped \\u0041 pack", "future_extension":true},\r\n  "version": {"version":"1.0.0"}\r\n}\r\n')]);
 const chooser=page.waitForEvent('filechooser');
 await page.getByRole('button',{name:'Import a test pack',exact:true}).click();
 await (await chooser).setFiles({name:'original escaped.json',mimeType:'application/json',buffer:original});
 await expect(page.getByRole('button',{name:'Download original original escaped.json',exact:true})).toBeVisible();
 const sourceDownload=page.waitForEvent('download');await page.getByRole('button',{name:'Download original original escaped.json',exact:true}).click();
 const source=await sourceDownload;const {readFile}=await import('node:fs/promises');expect(await readFile((await source.path())!)).toEqual(original);
 await page.getByRole('button',{name:'Settings',exact:true}).click();await page.getByRole('button',{name:/^Export (?:selected version \/ result|tests and agent instructions|agent and tests)$/}).click();
 await expect(page.getByRole('button',{name:'Download exact export',exact:true})).toBeVisible();
 const exportedDownload=page.waitForEvent('download');await page.getByRole('button',{name:'Download exact export',exact:true}).click();
 const exported=JSON.parse(await readFile((await (await exportedDownload).path())!,'utf8'));
 expect(exported.format).toBe('agentclash-evaluation-v2');expect(exported.manifest.completeness).toBe('exact');expect(exported.selection.scope).toBe('selected');expect(exported.runs).toHaveLength(0);
 expect(Buffer.from(exported.evidence.find((e:{reference:{kind:string}})=>e.reference.kind==='original').value.content,'base64')).toEqual(original);
 await page.getByRole('button',{name:'Close',exact:true}).last().click();
 for(const width of [320,390,1280]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await page.screenshot({path:info.outputPath(`original-${width}.png`),fullPage:true});}
 expect((await evidence(page)).calls).toHaveLength(0);
});
