/**
 * DEBUG: Tests if raw Response with Set-Cookie header works in Next.js 16.
 */
export async function GET() {
  const html = `<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0;url=/api/debug/read-cookie"></head><body>redirecting...</body></html>`;

  return new Response(html, {
    status: 200,
    headers: {
      "Content-Type": "text/html",
      "Set-Cookie": "debug_test=hello123; Path=/; Max-Age=3600; HttpOnly; SameSite=Lax",
    },
  });
}
