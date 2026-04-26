import {
  NextResponse,
  type NextFetchEvent,
  type NextRequest,
} from "next/server";
import { authkitMiddleware } from "@workos-inc/authkit-nextjs";

const authkit = authkitMiddleware();

const WORKSPACE_ROOT_PATTERN = /^\/workspaces\/[^/]+\/?$/;

export default function middleware(
  request: NextRequest,
  event: NextFetchEvent,
) {
  if (
    request.nextUrl.pathname === "/docs" ||
    request.nextUrl.pathname.startsWith("/docs/") ||
    request.nextUrl.pathname === "/docs-md" ||
    request.nextUrl.pathname.startsWith("/docs-md/") ||
    request.nextUrl.pathname === "/llms.txt" ||
    request.nextUrl.pathname === "/llms-full.txt" ||
    request.nextUrl.pathname === "/share" ||
    request.nextUrl.pathname.startsWith("/share/")
  ) {
    return NextResponse.next();
  }

  if (WORKSPACE_ROOT_PATTERN.test(request.nextUrl.pathname)) {
    const url = request.nextUrl.clone();
    url.pathname = `${request.nextUrl.pathname.replace(/\/$/, "")}/runs`;
    return NextResponse.redirect(url, 307);
  }

  return authkit(request, event);
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|og-image.png|twitter-image.png|favicon-16x16.png|favicon-32x32.png|favicon-96x96.png|apple-touch-icon.png|robots.txt|sitemap.xml).*)",
  ],
};
