import {
  NextResponse,
  type NextFetchEvent,
  type NextRequest,
} from "next/server";
import { authkitMiddleware } from "@workos-inc/authkit-nextjs";

const authkit = authkitMiddleware();

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
    request.nextUrl.pathname === "/llms-full.txt"
  ) {
    return NextResponse.next();
  }

  return authkit(request, event);
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|og-image.png|twitter-image.png|favicon-16x16.png|favicon-32x32.png|favicon-96x96.png|apple-touch-icon.png|robots.txt|sitemap.xml).*)",
  ],
};
