import { test, expect, type Page } from "@playwright/test";
const api = `http://127.0.0.1:${process.env.VIBE_BROWSER_API_PORT || "55441"}`;
const workspace = "a8000000-0000-4000-8000-000000000003";
const blueprint = { slug: "share-tests", name: "Selected report tests", difficulty: "easy", instructions: "{{question}}", cases: [{ key: "case-1", payload: { question: "May I return this?" }, expectations: [{ key: "expected_behavior", kind: "text", value: "Explain the return policy." }] }], validators: [{ key: "answer", type: "regex_match", target: "final_output", expected_from: "literal:.+" }], judges: [{ key: "behavior", mode: "assertion", assertion: "The response meets this case's expected_behavior and the following shared rules:\n\nNever claim to process a refund.", context_from: ["case.expectations.expected_behavior"] }], dimensions: [{ key: "answer", source: "validators", validators: ["answer"] }, { key: "behavior", source: "llm_judge", judge_key: "behavior" }] };
async function login(page: Page, user?: string) { if (user)
    await page.request.get(`http://127.0.0.1:55442/__fixture/identity?user=${user}`); await page.goto("/auth/login?returnTo=%2Fvibe-evals"); await page.getByRole("button", { name: "Continue with AgentClash" }).click();
    // A viewer has no workspace to start projects in, so the entry shows a notice instead of the picker.
    await expect(page.locator(".vibe-sidebar-workspace").or(page.getByText("You need a workspace to start a project.", { exact: false }))).toBeVisible(); }
test("selected copy survives private cleanup with independent member/viewer navigation and capability lifecycle", async ({ page, browser }) => {
    const errors: string[] = [];
    page.on("pageerror", e => errors.push(e.message));
    let ownerToken = "";
    page.on("request", r => { if (r.url().startsWith(api) && r.headers().authorization)
        ownerToken = r.headers().authorization; });
    await login(page);
    await expect(page.getByRole("button", { name: /Build an agent/ })).toBeEnabled(); await expect(page.getByLabel("Workspace for new projects")).toHaveCount(0);
    await page.getByRole("button", { name: /Improve an existing agent/ }).click();
    await expect(page).toHaveURL(/start=improve/);
    await page.request.post(`${api}/__fixture/control`, { data: { reset_calls: true } });
    const before = { calls: [] };
    await page.getByRole("radio", { name: "Test pack", exact: true }).click();
    const choose = page.waitForEvent("filechooser");
    await page.getByRole("button", { name: "Choose a file", exact: true }).click();
    await (await choose).setFiles({ name: "tests.json", mimeType: "application/json", buffer: Buffer.from(JSON.stringify(blueprint)) });
    await expect(page.getByRole("tab", { name: "Tests", exact: true })).toBeVisible();
    const session = new URL(page.url()).searchParams.get("session")!;
    await page.getByRole("tab", { name: "Results", exact: true }).click();
    await page.locator("summary").filter({ hasText: /^Import SDK report$/ }).click();
    const source = JSON.stringify({ schema_version: 1, report_id: "PRIVATE_REPORT", generated_at: "2026-10-26T12:00:00Z", runner: { name: "agentclash-evals", version: "1" }, summary: { total: 1, passed: 0, failed: 1, skipped: 0, errored: 0 }, cases: [{ case: { case_id: "PRIVATE_CASE", name: "PRIVATE_LABEL" }, status: "failed", metrics: [{ key: "PRIVATE_METRIC", name: "PRIVATE_NAME", passed: false, score: 0.2, reason: "PRIVATE_REASON" }], agent_result: { input: "PRIVATE_INPUT", output: "PRIVATE_OUTPUT" } }], exit_code: 1 }).replace('"score":0.2', '"score":0.12345678901234567890123456789');
    await page.getByRole("region", { name: "Import SDK report", exact: true }).locator('input[type="file"]').setInputFiles({ name: "report.json", mimeType: "application/json", buffer: Buffer.from(source) });
    await page.getByLabel("Test for PRIVATE_CASE — PRIVATE_LABEL").selectOption({ index: 1 });
    const imported = page.waitForResponse(r => r.url().endsWith(`/sessions/${session}/reports`) && r.request().method() === "POST");
    await page.getByRole("button", { name: "Import historical report", exact: true }).click();
    const importedResponse = await imported;
    expect(importedResponse.ok(), await importedResponse.text()).toBe(true);
    await page.locator("summary").filter({ hasText: /^Create a selected workspace report$/ }).click();
    await page.getByRole("button", { name: "Preview selected copy", exact: true }).click();
    const preview = page.getByRole("region", { name: "Exact report preview" });
    await expect(preview).toContainText("0.12345678901234567890123456789");
    await expect(preview).not.toContainText("PRIVATE_");
    const raw = page.getByRole("checkbox", { name: "Include source text (may contain private details)" }).first();
    await raw.check();
    await expect(preview).toHaveCount(0);
    await page.getByRole("button", { name: "Preview selected copy", exact: true }).click();
    await expect(preview).toContainText("PRIVATE_REASON");
    await expect(preview).not.toContainText("PRIVATE_INPUT");
    await raw.uncheck();
    await page.getByRole("button", { name: "Preview selected copy", exact: true }).click();
    await expect(preview).not.toContainText("PRIVATE_");
    await page.getByRole("button", { name: "Create workspace report", exact: true }).click();
    const open = page.getByRole("link", { name: "Open workspace report", exact: true });
    await expect(open).toBeVisible();
    const detail = (await open.getAttribute("href"))!;
    const snapshot = detail.split("/").at(-1)!;
    await open.click();
    await expect(page.getByRole("article", { name: "Selected report" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Open public report" })).toHaveCount(0);
    const viewerContext = await browser.newContext(), memberContext = await browser.newContext();
    try {
        const viewer = await viewerContext.newPage(), member = await memberContext.newPage();
        viewer.on("pageerror", e => errors.push(e.message));
        member.on("pageerror", e => errors.push(e.message));
        await login(viewer, "a8000000-0000-4000-8000-000000000005");
        await viewer.goto(`/workspaces/${workspace}/reports`);
        await viewer.getByRole("link", { name: /Selected report ·/ }).click();
        await expect(viewer.getByRole("article", { name: "Selected report" })).toBeVisible();
        await expect(viewer.getByRole("button", { name: "Append annotation" })).toHaveCount(0);
        await expect(viewer.getByRole("button", { name: "Create expiring public link" })).toHaveCount(0);
        await login(member, "a8000000-0000-4000-8000-000000000004");
        await member.goto(detail);
        await member.getByLabel("New annotation").fill("The original grade remains unchanged.");
        const noteSaved = member.waitForResponse(r => r.url().endsWith(`/report-snapshots/${snapshot}/notes`) && r.request().method() === "POST");
        await member.getByRole("button", { name: "Append annotation", exact: true }).click();
        expect((await noteSaved).status()).toBe(201);
        await expect(member.getByRole("region", { name: "Workspace annotations" }).getByRole("article")).toContainText("The original grade remains unchanged.");
        await member.getByRole("button", { name: "Create expiring public link", exact: true }).click();
        const publicLink = member.getByRole("link", { name: "Open public report" });
        await expect(publicLink).toBeVisible();
        const publicPath = (await publicLink.getAttribute("href"))!;
        const publicPage = await browser.newPage();
        try {
            const ssr = await publicPage.goto(publicPath);
            expect(ssr?.headers()["cache-control"]).toContain("no-store");
            await expect(publicPage.getByRole("article", { name: "Selected report" })).toContainText("0.12345678901234567890123456789");
            await expect(publicPage.locator("body")).not.toContainText("PRIVATE_");
            await expect(publicPage.locator("body")).not.toContainText("The original grade remains unchanged.");
            expect(await publicPage.locator('meta[name="robots"]').getAttribute("content")).toContain("noindex");
            // Production delete admission followed by actual cleanup, then reload both independent routes.
            const activity = await (await page.request.get(`${api}/__fixture/evidence?session=${session}`)).json();
            expect(activity.calls || []).toEqual(before.calls || []);
            expect(activity.session.operations).toHaveLength(0);
            const current = await (await page.request.get(`${api}/v1/vibe/sessions/${session}`, { headers: { Authorization: ownerToken } })).json();
            const deleted = await page.request.delete(`${api}/v1/vibe/sessions/${session}?revision=${current.revision}`, { headers: { Authorization: ownerToken } });
            expect(deleted.ok(), await deleted.text()).toBe(true);
            const cleanup = await page.request.post(`${api}/__fixture/report-cleanup?session=${session}`);
            expect(await cleanup.json()).toEqual({ raw_reports: 0 });
            await viewer.reload();
            await expect(viewer.getByRole("article", { name: "Selected report" })).toContainText("0.12345678901234567890123456789");
            await expect(viewer.getByRole("region", { name: "Workspace annotations" })).toContainText("The original grade remains unchanged.");
            await publicPage.reload();
            await expect(publicPage.getByRole("article", { name: "Selected report" })).toBeVisible();
            const links = await (await page.request.get(`${api}/v1/vibe/workspaces/${workspace}/report-snapshots/${snapshot}/links`, { headers: { Authorization: ownerToken } })).json();
            await page.request.post(`${api}/__fixture/report-expire?link=${links[0].id}`);
            await publicPage.reload();
            await expect(publicPage.getByRole("article", { name: "Selected report" })).toHaveCount(0);
            await member.reload();
            await expect(member.getByRole("region", { name: "Public report links" })).toContainText("Expired");
            await member.getByRole("button", { name: "Create expiring public link" }).click();
            await expect(publicLink).toBeVisible();
            const replacement = (await publicLink.getAttribute("href"))!;
            expect(replacement).not.toBe(publicPath);
            await publicPage.goto(replacement);
            await expect(publicPage.getByRole("article", { name: "Selected report" })).toBeVisible();
            await member.getByRole("button", { name: "Revoke link", exact: true }).click();
            await expect(member.getByRole("region", { name: "Public report links" })).toContainText("Revoked");
            await publicPage.reload();
            await expect(publicPage.getByRole("article", { name: "Selected report" })).toHaveCount(0);
            for (const width of [320, 768, 1440]) {
                await viewer.setViewportSize({ width, height: 900 });
                expect(await viewer.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
            }
        }
        finally {
            await publicPage.close();
        }
    }
    finally {
        await viewerContext.close();
        await memberContext.close();
    }
    expect(errors).toEqual([]);
});
