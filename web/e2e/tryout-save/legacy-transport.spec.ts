import { expect, test } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile } from "node:fs/promises";
import path from "node:path";
import ts from "typescript";

function listen(server: Server): Promise<string> {
 return new Promise(resolve => server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Expected loopback address");
  resolve(`http://127.0.0.1:${address.port}`);
 }));
}
function close(server: Server): Promise<void> { return new Promise(resolve => server.close(() => resolve())); }

// Real cross-origin browser fetch using the shipped transport source. No route
// interception, provider, cookies, production endpoint or alternate write path.
test("old CORS rejects unconditional credentials; negotiated legacy journey succeeds", async ({ page }) => {
 const calls: { method: string; path: string; body?: Record<string, unknown> }[] = [];
 let origin = "";
 const apiServer = createServer(async (req, res) => {
  res.setHeader("Access-Control-Allow-Origin", origin);
  res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
  res.setHeader("Access-Control-Allow-Headers", "Content-Type, Accept");
  // Intentionally no Access-Control-Allow-Credentials, like the older API.
  if (req.method === "OPTIONS") { res.writeHead(204).end(); return; }
  let body = ""; for await (const chunk of req) body += chunk;
  calls.push({ method: req.method!, path: req.url!, ...(body ? { body: JSON.parse(body) } : {}) });
  res.setHeader("Content-Type", "application/json"); res.setHeader("Cache-Control", "no-store");
  if (req.url === "/v1/agent-tryout-templates") { res.end(JSON.stringify({ items: [] })); return; }
  if (req.url?.endsWith("/claim")) { res.writeHead(500).end(); return; }
  if (req.url?.endsWith("/turns")) { res.writeHead(202).end(JSON.stringify({ status: "submitted" })); return; }
  if (req.url?.includes("/events")) { res.end(JSON.stringify({ events: [] })); return; }
  res.writeHead(req.method === "POST" ? 201 : 200).end(JSON.stringify({ id: "legacy-result", status: "completed", save_eligible: true }));
 });
 const api = await listen(apiServer);
 const modules = new Map<string, string>();
 for (const name of ["agent-tryouts", "client", "errors"]) {
  const source = (await readFile(path.resolve("src/lib/api", `${name}.ts`), "utf8")).replaceAll("process.env.NEXT_PUBLIC_API_URL", JSON.stringify(api));
  modules.set(`/${name}`, ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } }).outputText);
 }
 const webServer = createServer((req, res) => {
  const moduleSource = modules.get(req.url!);
  res.setHeader("Content-Type", moduleSource ? "text/javascript" : "text/html");
  res.end(moduleSource ?? "<!doctype html><title>Local legacy tryout fixture</title><main>Legacy transport fixture</main>");
 });
 origin = await listen(webServer);
 try {
  await page.goto(origin);
  const unconditionalFailed = await page.evaluate(async api => {
   try { await fetch(`${api}/v1/agent-tryouts`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json" }, body: "{}" }); return false; }
   catch { return true; }
  }, api);
  expect(unconditionalFailed).toBe(true);
  expect(calls.filter(call => call.method === "POST")).toHaveLength(0);
  const result = await page.evaluate(async origin => {
   // Native imports keep this fixture on the actual production source boundary.
   const transport = await import(`${origin}/agent-tryouts`);
   const { createApiClient } = await import(`${origin}/client`);
   const api = createApiClient();
   const created = await transport.createAnonymousAgentTryout(api, { template_slug: "meeting-minutes", input: { notes: "Synthetic legacy result" } });
   const read = await transport.getPublicAgentTryout(api, created.id);
   const events = await transport.getPublicAgentTryoutEvents(api, created.id);
   const turn = await transport.submitAgentTryoutTurn(api, created.id, { end: true });
   let refusedClaim = false;
   try { await transport.claimAgentTryout(api, created.id, "workspace"); } catch { refusedClaim = true; }
   return { created, read, events, turn, refusedClaim };
  }, origin);
  expect(result.created.save_eligible).toBe(false); expect(result.read.save_eligible).toBe(false);
  expect(result.events.events).toEqual([]); expect(result.turn.status).toBe("submitted"); expect(result.refusedClaim).toBe(true);
  expect(calls.filter(call => call.path === "/v1/agent-tryout-templates")).toHaveLength(5);
  expect(calls.filter(call => call.path.endsWith("/claim"))).toHaveLength(0);
  expect(calls.filter(call => call.method === "POST")).toEqual([
   { method: "POST", path: "/v1/agent-tryouts", body: { template_slug: "meeting-minutes", input: { notes: "Synthetic legacy result" } } },
   { method: "POST", path: "/v1/agent-tryouts/legacy-result/turns", body: { end: true } },
  ]);
 } finally { await Promise.all([close(webServer), close(apiServer)]); }
});
