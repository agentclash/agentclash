import { withAuth } from "@workos-inc/authkit-nextjs";
import { createTryCliProxy } from "../../../../lib/try-cli/proxy";

export const runtime = "nodejs";
const proxy = createTryCliProxy({
  // Never use NEXT_PUBLIC_TRY_CLI_API_URL here: /api/try would proxy to itself.
  serviceUrl: process.env.TRY_CLI_API_URL,
  secret: process.env.TRY_CLI_PROXY_SECRET,
  production: process.env.NODE_ENV === "production",
  vercel: process.env.VERCEL === "1",
  userId: async () => (await withAuth()).user?.id,
});

type Context = { params: Promise<{ path?: string[] }> };
async function handle(req: Request, { params }: Context) {
  return proxy(req, (await params).path ?? []);
}
export { handle as GET, handle as POST, handle as DELETE };
