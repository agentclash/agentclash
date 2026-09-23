import { expect, test } from "@playwright/test";
import type { Session } from "../../src/lib/vibe";
import { actionConfig, actionModels } from "./interaction-fixture";

for (const width of [320, 1440]) {
  test(`preparation-only work can be kept or downloaded without an agent at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({width,height:900});
    const session:Session={id:"brief-session",revision:1,anonymous:true,operations:[],document:{test_journey:true,models:actionModels,requirements:[],messages:[{id:"m",role:"user",content:"I want an agent that converts PDFs to Markdown."}],artifacts:[{id:"brief-one",kind:"test_plan",title:"PDF conversion plan",agent_prompt:"",blueprint:null,accepted:false,source_message_id:"m",test_plan:{title:"PDF conversion plan",objective:"Preserve headings, tables and links.",scenarios:[{input:"A PDF containing a table",expected:"A Markdown table with the same content"}],evidence_needed:["The original PDF and the converted Markdown"],next_steps:["Build the converter, then save one input and output."],local_test_code:""}}]}};
    const posts:string[]=[];const errors:string[]=[];page.on("pageerror",e=>errors.push(e.message));
    await page.route("**/v1/vibe/**",async route=>{
      const path=new URL(route.request().url()).pathname;
      if(route.request().method()==="POST")posts.push(path);
      if(path.endsWith("/events"))return route.fulfill({contentType:"text/event-stream",body:": connected\n\n"});
      return route.fulfill({contentType:"application/json",body:JSON.stringify(path.endsWith("/config")?actionConfig:session)});
    });
    await page.goto(`/vibe-evals?session=${session.id}&agent=brief-one`);
    await expect(page.getByRole("heading",{name:"PDF conversion plan"})).toBeVisible();
    const download=page.waitForEvent("download");
    await page.getByRole("button",{name:"Download brief",exact:true}).click();
    expect((await download).suggestedFilename()).toBe("vibe-preparation-brief.json");
    await page.getByRole("button",{name:"Keep this brief",exact:true}).click();
    await expect(page.getByRole("dialog")).toContainText("No agent or results are needed.");
    const login=page.getByRole("link",{name:"Sign in to save your work"});
    expect(new URL((await login.getAttribute("href"))!,page.url()).searchParams.get("returnTo")).toContain("agent=brief-one");
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:info.outputPath(`keep-brief-${width}.png`)});
    expect(posts).toEqual([]);expect(errors).toEqual([]);
  });
}
