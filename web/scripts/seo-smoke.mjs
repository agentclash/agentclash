#!/usr/bin/env node
// One production build and anonymous crawl, with local deterministic upstreams.
import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import matter from "gray-matter";
import { startAuthFixture } from "../e2e/vibe-stack/auth-fixture.mjs";
import { runSeoCrawl } from "./seo-crawl.mjs";

const cwd = fileURLToPath(new URL("..", import.meta.url));
const webPort = 43190;
const authPort = 43191;
const webOrigin = `http://127.0.0.1:${webPort}`;
const publications = [1, 2].map((number) => {
  const id = `a8000000-0000-4000-8000-00000000000${number}`;
  return {
    publication: { id, resource_type: "agent_tryout", canonical_path: `/publications/${id}`, created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-02T00:00:00Z" },
    resource: { type: "agent_tryout", template_slug: `seo-fixture-${number}`, status: "completed", redaction_status: "passed", input_snapshot: { task: "Classify the fixture ticket" }, summary: { message: "Fixture completed" }, artifacts: [] },
  };
});
const template = {
  slug: "seo-fixture", name: "SEO fixture tryout", description: "A deterministic anonymous tryout template.",
  available: true, anonymous_enabled: true, input_schema: { type: "object", properties: { task: { type: "string" } } },
  tool_policy: {}, runtime: {}, evaluation_spec: {}, default_model_policy: {},
  max_input_bytes: 4096, max_duration_seconds: 60, max_cost_usd: 1,
};
const unexpected = [];
const requests = new Set();
const api = createServer((req, res) => {
  const url = new URL(req.url, "http://127.0.0.1");
  requests.add(url.pathname + url.search);
  const json = (status, value) => { res.writeHead(status, { "content-type": "application/json" }); res.end(JSON.stringify(value)); };
  if (req.method === "GET") {
    if (url.pathname === "/public/publications") {
      // Deliberately paginate even when the caller requests a larger limit.
      const cursor = url.searchParams.get("cursor");
      if (!cursor) return json(200, { items: [publications[0]], next_cursor: "fixture-page-2" });
      if (cursor === "fixture-page-2") return json(200, { items: [publications[1]] });
    }
    const publication = publications.find((item) => url.pathname === `/public/publications/${item.publication.id}`);
    if (publication) return json(200, publication);
    if (url.pathname === "/v1/agent-tryout-templates") return json(200, { items: [template] });
    if (url.pathname === "/v1/tool-library") return json(200, { items: [{ slug: "filesystem", name: "Filesystem", category: "Tools", description: "Read fixture files." }] });
  }
  unexpected.push(`${req.method} ${url.pathname}${url.search}`);
  json(500, { error: "Unexpected SEO fixture request" });
});
const closeApi = () => new Promise((resolve) => { api.close(resolve); api.closeAllConnections(); });
let closeAuth;
let child;
let childExit;
let aborted = false;
function startNext(args, env) {
  if (aborted) throw new Error("SEO smoke interrupted");
  child = spawn(process.execPath, [path.join(cwd, "node_modules/next/dist/bin/next"), ...args], { cwd, env, stdio: "inherit" });
  childExit = new Promise((resolve, reject) => { child.once("error", reject); child.once("exit", (code, signal) => resolve({ code, signal })); });
  // Keep spawn failures observed while the readiness loop runs.
  childExit.catch(() => {});
  return childExit;
}
async function stopChild() {
  if (!child || child.exitCode !== null || child.signalCode !== null) return;
  child.kill("SIGTERM");
  const timeout = setTimeout(() => child?.kill("SIGKILL"), 5000);
  try { await childExit; } finally { clearTimeout(timeout); }
}
const abort = () => { aborted = true; child?.kill("SIGTERM"); process.exitCode = 1; };
process.once("SIGINT", abort);
process.once("SIGTERM", abort);

try {
  // Next loads these implicitly. Refuse them rather than accidentally consuming local secrets.
  for (const filename of [".env", ".env.local", ".env.production", ".env.production.local"]) {
    if (existsSync(path.join(cwd, filename))) throw new Error(`SEO smoke requires a clean environment: remove ${filename} from this checkout first`);
  }
  await new Promise((resolve, reject) => { api.once("error", reject); api.listen(0, "127.0.0.1", resolve); });
  closeAuth = await startAuthFixture(authPort, webOrigin);
  const apiOrigin = `http://127.0.0.1:${api.address().port}`;
  // Only host execution settings are inherited; all app configuration is fixture-owned.
  const env = Object.fromEntries(["PATH", "HOME", "TMPDIR", "SystemRoot", "CI", "SSL_CERT_FILE", "SSL_CERT_DIR"].filter((key) => process.env[key] !== undefined).map((key) => [key, process.env[key]]));
  Object.assign(env, {
    NODE_ENV: "production", NEXT_TELEMETRY_DISABLED: "1", API_URL: apiOrigin, NEXT_PUBLIC_API_URL: apiOrigin,
    TRY_CLI_API_URL: apiOrigin, NEXT_PUBLIC_TRY_CLI_API_URL: apiOrigin,
    WORKOS_API_HOSTNAME: "127.0.0.1", WORKOS_API_PORT: String(authPort), WORKOS_API_HTTPS: "false",
    WORKOS_CLIENT_ID: "client_seo_fixture", WORKOS_API_KEY: "sk_test_seo_fixture",
    WORKOS_COOKIE_PASSWORD: "seo-fixture-cookie-password-not-a-secret-1234567890",
    NEXT_PUBLIC_WORKOS_REDIRECT_URI: `${webOrigin}/auth/callback`, NEXT_PUBLIC_POSTHOG_KEY: "", ANALYTICS_REQUIRED: "false",
  });
  const build = await startNext(["build"], env);
  if (build.code !== 0) throw new Error(`Next production build failed (${build.code ?? build.signal})`);
  startNext(["start", "--hostname", "127.0.0.1", "--port", String(webPort)], env);
  let ready = false;
  for (let attempt = 0; attempt < 120; attempt++) {
    if (aborted) throw new Error("SEO smoke interrupted");
    if (child.exitCode !== null || child.signalCode !== null) throw new Error("Next exited before readiness");
    try {
      const response = await fetch(`${webOrigin}/sitemap.xml`, { redirect: "manual", signal: AbortSignal.timeout(2000) });
      if (response.status === 200) { ready = true; break; }
    } catch { /* retry until the production listener is ready */ }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  if (!ready) throw new Error("Next production server did not become ready");
  const samplePaths = readdirSync(path.join(cwd, "content/benchmarks"))
    .filter((name) => name.endsWith(".mdx") && matter(readFileSync(path.join(cwd, "content/benchmarks", name), "utf8")).data.sample === true)
    .map((name) => `/benchmarks/${name.slice(0, -4)}`);
  const result = await runSeoCrawl({ baseUrl: webOrigin, samplePaths });
  for (const publication of publications) {
    if (!result.publicationPaths.includes(publication.publication.canonical_path)) result.failures.push(`Missing paginated publication ${publication.publication.id}`);
  }
  // Confirm that anonymous tryout SSR consumed the nonempty fixture, even if its loader swallows upstream errors.
  const tryouts = await (await fetch(`${webOrigin}/tryouts`)).text();
  if (!tryouts.includes(template.name)) result.failures.push("Tryout initial HTML omitted the fixture template");
  for (const endpoint of ["/v1/agent-tryout-templates", "/v1/tool-library"]) {
    if (![...requests].some((request) => request === endpoint)) result.failures.push(`Fixture endpoint was not exercised: ${endpoint}`);
  }
  result.failures.push(...unexpected.map((request) => `Unexpected API usage (bundled demos must stay local): ${request}`));
  if (result.failures.length) throw new Error(result.failures.join("\n"));
  console.log(`Production SEO smoke passed: ${result.paths.length} static URLs and ${result.publicationPaths.length} paginated publications.`);
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  await stopChild();
  if (closeAuth) await closeAuth();
  await closeApi();
  process.removeListener("SIGINT", abort);
  process.removeListener("SIGTERM", abort);
}
