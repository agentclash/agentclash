// Comparable production-only local lab probe. No inference or external traffic.
import { createRequire } from "node:module";
import { writeFile, readFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
import { recordOwned } from "../../web/e2e/vibe-stack/ownership.mjs";
const require = createRequire(path.resolve("web/package.json"));
const { chromium } = require("playwright");
const [base, commit, output] = process.argv.slice(2);
if (!base || !commit || !output || new URL(base).hostname !== "127.0.0.1") throw Error("Explicit loopback production URL, source commit and new output required");
const models = { assistant: "openai/gpt-4.1-mini", target: "openai/gpt-4.1-mini", evaluator: "openai/gpt-4.1-mini" };
const cases = Array.from({length:100},(_,i)=>({key:`case-${i}`,payload:{question:`Classify customer message ${i}.`},expectations:[{key:"expected_behavior",kind:"text",value:"Label as Spam."}]}));
const session = { id:"perf-project",title:"Long customer email evaluation",revision:1,event_cursor:1,anonymous:true,document:{format_version:1,models,requirements:[],test_journey:true,evaluation:{id:"perf-project",chat_id:"chat",door:"build"},active_artifact_id:"prototype",artifacts:[{id:"prototype",kind:"test_suite",title:"Customer email sorter",agent_prompt:"Suggest a label for each email.",accepted:true,source_message_id:"request",proposal_message_id:"prepared",blueprint:{judges:[{context_from:["case.expectations.expected_behavior"]}],cases}}],messages:[{id:"request",role:"user",content:"Sort customer emails.",operation_id:"prepare"},...Array.from({length:60},(_,i)=>({id:`message-${i}`,role:i%2?"assistant":"user",content:`Retained context ${i}: classify only the supplied email. `.repeat(8),operation_id:"prepare"})),{id:"prepared",role:"assistant",content:"",operation_id:"prepare",artifact_id:"prototype"}],build:{cycle_id:"cycle",phase:"results",clarifications_used:0,artifact_id:"prototype",check_id:"check"}},operations:[{id:"prepare",kind:"message",state:"COMPLETED",billing:"SETTLED",models,max_cost_nano_usd:0,actual_cost_nano_usd:0,results:[]},{id:"check",kind:"check",state:"COMPLETED",billing:"SETTLED",models,source:{kind:"prompt",artifact_id:"prototype",label:"Customer email sorter"},max_cost_nano_usd:0,actual_cost_nano_usd:0,results:cases.map(c=>({case_key:c.key,title:c.payload.question,version:"prototype",input:c.payload,expected:"Label as Spam.",output:"Suggested label: Spam",verdict:"PASS",checks:[{key:"behavior",verdict:"PASS",evidence:"Correct label."}]})),scorecard:{total:100,passed:100,failed:0,unknown:0,evaluated:100,coverage:1,pass_rate:1}}] };
const config={enabled:true,two_door:true,defaults:models,availability:{guest:{available:true},workspace:{available:true}},models:[{id:models.assistant,name:"Fixture model",input_nano_per_token:0,output_nano_per_token:0}]};
const fixtureSHA256=createHash("sha256").update(JSON.stringify({config,session})).digest("hex");
await recordOwned("performance_browser",`${commit}:${base}`);
const browser=await chromium.launch({headless:true,executablePath:process.env.VIBE_TEST_CHROMIUM});
const browserSession=await browser.newBrowserCDPSession();
const processes=await browserSession.send("SystemInfo.getProcessInfo");
const browserPID=processes.processInfo.find(p=>p.type==="browser")?.id;
await recordOwned("performance_browser",`${commit}:${base}`,{phase:"started",child_pid:browserPID});
const browserStart = browserPID ? await readFile(`/proc/${browserPID}/stat`, "utf8") : null;
await recordOwned("performance_samples",output);
const samples=[];
try {
 for(const width of [1440,390]) for(let sample=1;sample<=5;sample++) {
  const context=await browser.newContext({viewport:{width,height:900},colorScheme:"light",reducedMotion:"reduce"});
  const page=await context.newPage();
  const errors=[],blocked=[],writes=[];
  page.on("pageerror",e=>errors.push(e.message));
  await page.route("**/*",async route=>{
   const url=new URL(route.request().url());
   if(url.pathname.startsWith("/v1/vibe/")){
    const headers={"Access-Control-Allow-Origin":new URL(base).origin,"Access-Control-Allow-Credentials":"true"};
    if(route.request().method()==="OPTIONS") return route.fulfill({status:204,headers:{...headers,"Access-Control-Allow-Headers":"Content-Type,Authorization","Access-Control-Allow-Methods":"GET,OPTIONS"}});
    if(route.request().method()!=="GET"){writes.push(url.pathname);return route.fulfill({status:400,headers,json:{error:{message:"No writes in performance fixture"}}});}
    if(url.pathname.endsWith("/config"))return route.fulfill({headers,json:config});
    if(url.pathname.endsWith("/sessions")||url.pathname.endsWith("/evaluations"))return route.fulfill({headers,json:[session]});
    if(url.pathname.endsWith("/saved-checks"))return route.fulfill({headers,json:[]});
    if(url.pathname.endsWith("/events"))return route.fulfill({headers,contentType:"text/event-stream",body:"event: connected\ndata: {}\n\n"});
    return route.fulfill({headers,json:session});
   }
   if(url.origin!==new URL(base).origin){blocked.push(url.origin+url.pathname);return route.abort();}
   return route.continue();
  });
  const cdp=await context.newCDPSession(page);
  await cdp.send("Network.enable");await cdp.send("Network.setCacheDisabled",{cacheDisabled:true});
  await cdp.send("Network.emulateNetworkConditions",{offline:false,latency:150,downloadThroughput:200000,uploadThroughput:93750,connectionType:"cellular3g"});
  await cdp.send("Emulation.setCPUThrottlingRate",{rate:4});
  const started=performance.now();
  await page.goto(`${base}/vibe-evals?session=perf-project&agent=prototype&view=results&run=check`,{waitUntil:"domcontentloaded",timeout:120000});
  await page.getByRole("tab",{name:"Results",exact:true}).waitFor({timeout:120000});
  if(width<1280){const guide=page.getByRole("button",{name:"Guide",exact:true});if(await guide.isVisible())await guide.click();}
  const composer=page.getByRole("textbox",{name:"Message Vibe Evals",exact:true});
  await composer.waitFor({state:"visible",timeout:30000});
  const readyMS=performance.now()-started;
  await composer.focus();
  const inputStarted=performance.now();await composer.pressSequentially("Review this retained example.");
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  const keyboardMS=performance.now()-inputStarted;
  const metrics=await page.evaluate(()=>({fcpMS:performance.getEntriesByName("first-contentful-paint")[0]?.startTime??null,scripts:performance.getEntriesByType("resource").filter(r=>r.initiatorType==="script").map(r=>({name:new URL(r.name).pathname,encodedBodySize:r.encodedBodySize,transferSize:r.transferSize})),overflow:document.documentElement.scrollWidth>innerWidth+1}));
  if(errors.length||writes.length)throw Error(JSON.stringify({errors,writes}));
  samples.push({width,sample,readyMS,keyboardMS,...metrics,blockedExternal:blocked});
  await context.close();
 }
 await writeFile(output,JSON.stringify({version:1,commit,base,fixtureSHA256,method:{build:"production",browser:browser.version(),platform:process.platform,viewportHeight:900,coldContextPerSample:true,cacheDisabled:true,cpuRate:4,latencyMS:150,downloadBytesPerSecond:200000,uploadBytesPerSecond:93750,cases:100,messages:62,theme:"light",reducedMotion:true,claim:"Local lab rendering and typing only; no model/network application accuracy or physical Mac claim"},samples},null,2),{flag:"wx"});
} finally {
 await browser.close();
 let absence = false;
 if (browserPID) {
  try { const current = await readFile(`/proc/${browserPID}/stat`, "utf8"); absence = current.split(") ")[1].split(" ")[19] !== browserStart.split(") ")[1].split(" ")[19]; }
  catch (error) { if (error.code !== "ENOENT") throw error; absence = true; }
 }
 await recordOwned("performance_browser",`${commit}:${base}`,{phase:absence?"absent":"closed_unverified",child_pid:browserPID});
 if (!absence) throw Error("Owned browser exit could not be confirmed");
}
