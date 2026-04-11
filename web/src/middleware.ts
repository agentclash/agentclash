import { authkitMiddleware } from "@workos-inc/authkit-nextjs";

export default authkitMiddleware();

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|og-image.png|twitter-image.png|favicon-16x16.png|favicon-32x32.png|favicon-96x96.png|apple-touch-icon.png|robots.txt|sitemap.xml).*)",
  ],
};
