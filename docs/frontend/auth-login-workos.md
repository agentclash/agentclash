# AgentClash Login and WorkOS AuthKit

AgentClash uses WorkOS AuthKit for hosted authentication and keeps the visible
login entry point inside the AgentClash app at `/auth/login`.

## Current Approach

- `/auth/login` owns the AgentClash-branded login page and call to action.
- Submitting the login form calls `getSignInUrl({ returnTo })` and redirects to
  hosted AuthKit.
- `/auth/callback` continues to use `handleAuth`, so the WorkOS SDK owns the
  authorization-code exchange and session cookie handling.
- `returnTo` remains allowlisted through `sanitizeReturnTo`; unsupported or
  unsafe values fall back to `/dashboard`.

## WorkOS Research Notes

- WorkOS recommends hosted AuthKit as the quickest Next.js integration. Its
  callback route must match the configured redirect URI, and its SDK requires a
  strong cookie password of at least 32 characters.
- Hosted AuthKit keeps complex auth flows outside the app, including signup,
  password reset, email verification, SSO routing, MFA enrollment, bot
  protection, localization, and session handling.
- WorkOS branding supports logos, favicons, colors, copy, page layout, and
  custom CSS through the dashboard.
- The default hosted AuthKit domain is a generated `*.authkit.app` host. A custom
  production AuthKit domain can be configured through WorkOS DNS verification,
  but it is a paid WorkOS add-on.
- A fully custom, self-hosted login UI is possible through WorkOS User Management
  APIs, but it also means AgentClash must implement more auth states directly:
  password auth, signup, reset flows, verification codes, MFA challenges, org
  selection, custom email links, and session handoff.

## Production Checklist

- Configure WorkOS Redirect URI to the deployed callback:
  `https://www.agentclash.dev/auth/callback`.
- Configure the WorkOS sign-in endpoint to the deployed login page:
  `https://www.agentclash.dev/auth/login`.
- On Vercel, set `NEXT_PUBLIC_WORKOS_REDIRECT_URI` to the same www callback URL.
- Optionally set `WORKOS_COOKIE_DOMAIN=.agentclash.dev` so PKCE cookies work when
  users enter via the apex host (`agentclash.dev` redirects to www).
- Configure the WorkOS sign-out redirect location to an AgentClash-owned route.
- Use WorkOS dashboard branding to match AgentClash copy, assets, colors, and
  page layout while the app stays on the hosted AuthKit flow.
- Budget for the custom-domain add-on before promising an `auth.agentclash.dev`
  hosted AuthKit URL.

## Sources

- WorkOS AuthKit Next.js guide: https://workos.com/docs/authkit/nextjs
- WorkOS Hosted UI guide: https://workos.com/docs/authkit/hosted-ui
- WorkOS AuthKit branding: https://workos.com/docs/authkit/branding
- WorkOS AuthKit custom domain: https://workos.com/docs/custom-domains/authkit
- WorkOS pricing: https://workos.com/pricing
- WorkOS AuthKit authentication API reference:
  https://workos.com/docs/reference/authkit/authentication
