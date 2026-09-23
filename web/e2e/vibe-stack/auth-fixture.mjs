// Local identity provider for the real Next/AuthKit login + callback path.
// This server is started only by the isolated browser test launcher. No hosted
// accounts or secrets are used. The backend still uses DevelopmentAuthenticator.
import { createServer } from "node:http";
import { createHash, generateKeyPairSync, randomUUID, sign } from "node:crypto";

export const fixtureUser = "a8000000-0000-4000-8000-000000000001";
export async function startAuthFixture(port, webOrigin) {
  const { publicKey, privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
  const jwk = { ...publicKey.export({ format: "jwk" }), kid: "vibe-local", alg: "RS256", use: "sig" };
  const accessToken = () => {
    const encode = value => Buffer.from(JSON.stringify(value)).toString("base64url");
    const now = Math.floor(Date.now() / 1000);
    const unsigned = `${encode({ alg: "RS256", kid: "vibe-local" })}.${encode({ sub: fixtureUser, sid: "vibe-local-session", iat: now, exp: now + 3600 })}`;
    return `${unsigned}.${sign("RSA-SHA256", Buffer.from(unsigned), privateKey).toString("base64url")}`;
  };
  const codes = new Map();
  const server = createServer(async (req, res) => {
    const url = new URL(req.url, `http://127.0.0.1:${port}`);
    const json = (status, body) => { res.writeHead(status, { "Content-Type": "application/json" }); res.end(JSON.stringify(body)); };
    try {
      if (url.pathname.includes("jwks")) return json(200, { keys: [jwk] });
      if (url.pathname === "/user_management/authorize") {
        const redirect = new URL(url.searchParams.get("redirect_uri"));
        if (redirect.origin !== webOrigin || redirect.pathname !== "/auth/callback") return json(400, { error: "unknown callback" });
        const code = randomUUID(); codes.set(code, url.searchParams.get("code_challenge"));
        redirect.searchParams.set("state", url.searchParams.get("state")); redirect.searchParams.set("code", code);
        res.writeHead(302, { Location: redirect.href }); return res.end();
      }
      if (url.pathname === "/user_management/authenticate" && req.method === "POST") {
        let raw=""; for await (const chunk of req) raw+=chunk;
        const body=JSON.parse(raw);
        const challenge=codes.get(body.code); codes.delete(body.code);
        if (!challenge || createHash("sha256").update(body.code_verifier || "").digest("base64url") !== challenge) return json(400, { error: "invalid code verifier" });
        const token=accessToken();
        return json(200, { access_token:token, refresh_token:"local-test-only", authentication_method:"Password",
          user:{ object:"user", id:fixtureUser, email:"vibe-browser@example.invalid", email_verified:true, first_name:"Browser", last_name:"Test", created_at:new Date().toISOString(), updated_at:new Date().toISOString() } });
      }
      json(404, { error:"unsupported fixture endpoint", path:url.pathname });
    } catch { json(500,{error:"identity fixture error"}); }
  });
  await new Promise((resolve,reject)=>{server.once("error",reject);server.listen(port,"127.0.0.1",resolve);});
  return () => new Promise(resolve=>server.close(resolve));
}
