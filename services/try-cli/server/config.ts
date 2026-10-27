import { BlockList, isIP } from "node:net";

type Environment = Record<string, string | undefined>;

function invalid(name: string): never {
  // Never include configuration values: URLs can contain credentials.
  throw new Error(`Invalid or missing configuration: ${name}`);
}

export function loadConfig(env: Environment = process.env) {
  const production = env.NODE_ENV === "production";
  const number = (name: string, fallback: number, max: number, integer = false) => {
    const value = Number(env[name] ?? fallback);
    if (!Number.isFinite(value) || value <= 0 || value > max || (integer && !Number.isInteger(value))) invalid(name);
    return value;
  };
  const origin = (value: string, name: string) => {
    let url: URL;
    try { url = new URL(value); } catch { return invalid(name); }
    if (url.username || url.password || url.search || url.hash || url.pathname !== "/" ||
      !["http:", "https:"].includes(url.protocol) || (production && url.protocol !== "https:")) invalid(name);
    return url.origin;
  };
  const { E2B_API_KEY: e2bApiKey } = env;
  const proxySecret = env.TRY_CLI_PROXY_SECRET;
  const redisUrl = env.REDIS_URL || env.VALKEY_URL;
  const gatewayOrigin = env.TRY_CLI_GATEWAY_URL ? origin(env.TRY_CLI_GATEWAY_URL, "TRY_CLI_GATEWAY_URL") : undefined;
  if (production) {
    if (!e2bApiKey?.trim()) invalid("E2B_API_KEY");
    if (!proxySecret || Buffer.byteLength(proxySecret) < 32) invalid("TRY_CLI_PROXY_SECRET");
    if (!redisUrl) invalid("REDIS_URL");
    if (!gatewayOrigin) invalid("TRY_CLI_GATEWAY_URL");
    if (!env.TRY_CLI_CORS_ORIGINS?.trim()) invalid("TRY_CLI_CORS_ORIGINS");
    if (env.NODE_TLS_REJECT_UNAUTHORIZED === "0") invalid("NODE_TLS_REJECT_UNAUTHORIZED");
  }
  if (redisUrl) {
    let url: URL;
    try { url = new URL(redisUrl); } catch { return invalid("REDIS_URL"); }
    if (!url.hostname || url.search || url.hash || !["redis:", "rediss:"].includes(url.protocol) ||
      (production && url.protocol !== "rediss:")) invalid("REDIS_URL");
  }
  if (env.REDIS_TLS_CA_FILE && !redisUrl?.startsWith("rediss:")) invalid("REDIS_TLS_CA_FILE");
  const origins = (env.TRY_CLI_CORS_ORIGINS ?? "http://localhost:3000").split(",").map(s => origin(s.trim(), "TRY_CLI_CORS_ORIGINS"));
  const trustedProxies = new BlockList();
  for (const subnet of (env.TRY_CLI_TRUSTED_PROXY_CIDRS ?? "").split(",").filter(Boolean)) {
    const [ip, bits, extra] = subnet.trim().split("/");
    const family = isIP(ip!);
    const prefix = Number(bits);
    if (!family || bits === undefined || extra !== undefined || !Number.isInteger(prefix) || prefix < 1 || prefix > (family === 4 ? 32 : 128)) invalid("TRY_CLI_TRUSTED_PROXY_CIDRS");
    trustedProxies.addSubnet(ip!, prefix, family === 4 ? "ipv4" : "ipv6");
  }
  const codexModel = env.GW_CODEX_MODEL ?? "gpt-5-codex";
  if (!/^[a-zA-Z0-9._/-]+$/.test(codexModel)) invalid("GW_CODEX_MODEL");
  return {
    production, e2bApiKey, proxySecret, redisUrl, gatewayOrigin, origins, trustedProxies, codexModel,
    redisCaFile: env.REDIS_TLS_CA_FILE,
    port: number("PORT", 3001, 65535, true),
    anonMinutes: number("GW_ANON_MINUTES", 7, 60),
    authMinutes: number("GW_AUTH_MINUTES", 20, 60),
    sessionBudget: number("GW_SESSION_BUDGET_USD", 0.3, 100),
    maxSandboxes: number("TRY_CLI_MAX_CONCURRENT_SANDBOXES", 40, 1000, true),
    trialSeconds: Math.ceil(number("GW_TRIAL_RESET_HOURS", 24, 720) * 3600),
    dailyCeiling: number("GW_DAILY_CEILING_USD", 5, 1000),
    requestReserve: number("GW_REQUEST_RESERVE_USD", 0.05, 100),
    maxRequestBytes: number("GW_MAX_REQUEST_BYTES", 256 * 1024, 1024 * 1024, true),
    maxOutputTokens: number("GW_MAX_OUTPUT_TOKENS", 2048, 8192, true),
    shutdownMs: number("TRY_CLI_SHUTDOWN_MS", 45000, 120000, true),
    gatewayTimeoutMs: number("GW_REQUEST_TIMEOUT_MS", 120000, 600000, true),
    providerKeys: {
      openai: env.OPENAI_API_KEY, anthropic: env.ANTHROPIC_API_KEY,
      xai: env.XAI_API_KEY, openrouter: env.OPENROUTER_API_KEY,
    },
  };
}

export type Config = ReturnType<typeof loadConfig>;
