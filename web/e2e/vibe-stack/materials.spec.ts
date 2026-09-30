import { expect, test } from "@playwright/test";

const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT ?? "55441"}`;
const task = "Summarize meeting notes, including decisions and action owners.";
const notes = "At the planning meeting, Mira agreed to ship the prototype on Friday.";
test.beforeEach(async ({request}) => {
  test.skip(process.env.VIBE_BROWSER_MATERIALS !== "1", "Material Build fixture");
  await request.post(`${api}/__fixture/control`, {data:{reset_calls:true}});
});

async function saved(page: import("@playwright/test").Page) {
  const id = new URL(page.url()).searchParams.get("session");
  return (await page.request.get(`${api}/__fixture/evidence?session=${id}`)).json();
}
function pdf() {
  const stream = `BT /F1 12 Tf 20 100 Td (${notes}) Tj ET`;
  const objects = ["<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 600 200] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents 4 0 R >>", `<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`];
  let body = "%PDF-1.4\n"; const offsets = [0];
  objects.forEach((value, i) => { offsets.push(Buffer.byteLength(body)); body += `${i+1} 0 obj\n${value}\nendobj\n`; });
  const xref = Buffer.byteLength(body);
  body += `xref\n0 ${offsets.length}\n0000000000 65535 f \n${offsets.slice(1).map(n => `${n.toString().padStart(10,"0")} 00000 n \n`).join("")}trailer\n<< /Size ${offsets.length} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`;
  return Buffer.from(body);
}

for (const kind of ["paste", "pdf"] as const) test(`${kind}: material to actual output, checks, contact, reload and deletion`, async ({ page }, info) => {
  test.skip(kind === "pdf" && !process.env.VIBE_TEST_PDF_RUNTIME, "Real isolated PDF reader required");
  const errors: string[] = []; page.on("pageerror", e => errors.push(e.message));
  await page.goto("/vibe-evals");
  await page.getByRole("button", {name:/Build an agent/}).click();
  await page.getByRole("button", {name:"Add a file or text", exact:true}).click();
  if (kind === "paste") {
    await page.getByRole("tab", {name:"Paste text"}).click();
    await page.getByRole("textbox", {name:"Text to work on"}).fill(notes);
    await page.getByRole("button", {name:"Attach text", exact:true}).click();
  } else {
    await page.getByLabel("Choose one PDF").setInputFiles({name:"notes.pdf", mimeType:"application/pdf", buffer:pdf()});
  }
  await expect(page.locator(".vibe-material-chip-status")).toHaveText("Ready");
  await page.getByRole("button", {name:"Close material dialog",exact:true}).click();
  await page.reload();
  await expect(page.locator(".vibe-material-chip-status")).toHaveText("Ready");
  await page.getByRole("textbox", {name:"Message Vibe Evals",exact:true}).fill(task);
  await expect(page.getByRole("button", {name:"Send message",exact:true})).toBeEnabled();
  await page.getByRole("button", {name:"Send message",exact:true}).click();
  await expect.poll(async () => (await saved(page)).session.document.build?.phase, {timeout:90000}).toBe("results");
  await expect(page.getByRole("region",{name:"Your agent’s output"})).toContainText("Mira will ship the prototype on Friday");
  await page.getByText("Material used",{exact:true}).click();
  await page.getByText("Page 1",{exact:true}).click();
  await expect(page.getByRole("region",{name:"Your agent’s output"})).toContainText(notes);
  await page.getByText("Material used",{exact:true}).click();
  const state = await saved(page);
  expect(state.session.document.build.clarifications_used).toBe(0);
  expect(state.session.operations.map((o:{kind:string})=>o.kind)).toEqual(["message","playground","check"]);
  expect(state.session.operations.at(-1).results).toHaveLength(3);
  expect(state.session.document.artifacts[0].agent_prompt).not.toContain("Ana");
  expect(state.session.document.artifacts[0].agent_prompt).not.toContain("Mira");
  await expect(page.locator(".vibe-result-row")).toHaveCount(0);
  await page.getByRole("button",{name:"Discuss this with AgentClash",exact:true}).click();
  await expect(page.getByRole("dialog")).toContainText("Contact isn’t set up yet. You can copy your summary.");
  const summary=page.getByRole("textbox",{name:"Project summary",exact:true});
  await expect(summary).not.toHaveValue(/Mira|Ana/);
  for(const viewport of [{width:1440,height:900},{width:390,height:844},{width:360,height:800}]) {
    await page.setViewportSize(viewport);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await expect(page.getByLabel("Active evaluation")).toBeInViewport();
    await page.screenshot({path:info.outputPath(`${kind}-${viewport.width}.png`)});
  }
  await page.setViewportSize({width:1440,height:900});
  await page.getByRole("dialog").getByRole("button",{name:"Close",exact:true}).click();
  await page.locator(".vibe-output-download summary").click();
  const download=page.waitForEvent("download"); await page.getByRole("button",{name:"Markdown (.md)"}).click(); await download;
  await page.reload();
  expect((await saved(page)).session.operations).toHaveLength(3);
  await page.getByRole("button",{name:"Delete this project",exact:true}).click();
  await expect(page.getByRole("dialog")).toContainText("Separate workspace saves");
  await page.getByRole("button",{name:"Delete project permanently",exact:true}).click();
  await expect(page.getByRole("heading",{name:"Project removed",exact:true})).toBeVisible();
  await page.getByRole("button",{name:"Done",exact:true}).click();
  await expect(page.getByRole("button",{name:/Build an agent/})).toBeVisible();
  expect(errors).toEqual([]);
});

test("bad PDF offers paste without creating an agent or silently shortening input",async({page})=>{
  test.skip(!process.env.VIBE_TEST_PDF_RUNTIME,"Real reader required");
  await page.goto("/vibe-evals");await page.getByRole("button",{name:/Build an agent/}).click();
  await page.getByRole("button",{name:"Add a file or text",exact:true}).click();
  await page.getByLabel("Choose one PDF").setInputFiles({name:"broken.pdf",mimeType:"application/pdf",buffer:Buffer.from("%PDF-broken")});
  await expect(page.getByRole("alert").filter({hasText:"Paste its text instead"})).toBeVisible();
  expect((await saved(page)).session.operations).toHaveLength(0);
  await page.getByRole("button",{name:"Detach",exact:true}).click();
  await page.getByRole("tab",{name:"Paste text"}).click();
  await expect(page.getByRole("textbox",{name:"Text to work on",exact:true})).toBeEnabled();
});
