# Search Console + Bing verification & IndexNow

This is the runbook for two SEO features that are wired in code but need one-time
activation in external consoles and Vercel env. No code changes are required to
activate them.

Canonical host is **`https://www.agentclash.dev`** (with `www`) — set in
`web/src/app/layout.tsx` (`siteUrl`), `web/src/lib/docs.ts` (`DOCS_ORIGIN`), and
`web/src/app/robots.ts`. Every property/token below must use that exact host or
verification silently fails.

## 1. Google Search Console + Bing Webmaster verification

The HTML-meta-tag method is what the code implements (`lib/seo`
`webmasterVerification()`, rendered via `metadata.verification` in
`layout.tsx`). Do **not** use the DNS-TXT or file-upload methods — they bypass
this wiring.

### Get the tokens

- **Google Search Console** — <https://search.google.com/search-console> → add a
  **URL-prefix** property for `https://www.agentclash.dev` → choose **HTML tag**
  → copy only the `content` value (the token, not the whole `<meta>` tag). That
  value is `NEXT_PUBLIC_GSC_VERIFICATION`.
- **Bing Webmaster Tools** — <https://www.bing.com/webmasters> → add
  `https://www.agentclash.dev` → **HTML Meta Tag** → copy only the
  `msvalidate.01` `content` value. That value is `NEXT_PUBLIC_BING_VERIFICATION`.
  (Shortcut: after GSC verifies, Bing offers **Import from Google Search
  Console**, which can skip manual Bing verification entirely.)

### Set the env in Vercel + redeploy

Vercel Dashboard → web project → **Settings → Environment Variables** → add both
to **Production** (Preview optional; the tokens are public). Or via CLI from the
web project dir:

```bash
vercel env add NEXT_PUBLIC_GSC_VERIFICATION production
vercel env add NEXT_PUBLIC_BING_VERIFICATION production
vercel --prod   # NEXT_PUBLIC_ vars are build-time inlined — a fresh build is required
```

These are `NEXT_PUBLIC_` (inlined into client HTML) **on purpose** — webmaster
tokens are designed to live in public `<head>`. Never copy this pattern for a
real secret.

### Verify + submit

```bash
curl -s https://www.agentclash.dev/ | grep -E 'google-site-verification|msvalidate.01'
```

Both `<meta>` tags should appear with the exact tokens. Then click **Verify** in
each console. Post-verify: in GSC submit `sitemap.xml`, and use URL Inspection →
**Request Indexing** for the homepage and key pages; in Bing submit the same
sitemap (or rely on the GSC import).

## 2. IndexNow

IndexNow instantly notifies **Bing, Yandex, Naver, Seznam.cz, Yep** (NOT Google)
that content changed. Because Bing feeds Microsoft Copilot, faster Bing indexing
also helps Copilot answer freshness.

Already implemented in code:

- Key file at `web/public/265a46be97a2ce1f8891dd452d243327.txt` → served at
  `https://www.agentclash.dev/265a46be97a2ce1f8891dd452d243327.txt` (the
  middleware matcher excludes it so AuthKit doesn't intercept it).
- `web/src/lib/indexnow.ts` builds the URL list from the sitemap and POSTs to
  `api.indexnow.org`.
- `web/src/app/api/indexnow/route.ts` is the ping endpoint, protected by
  `CRON_SECRET`.
- `web/vercel.json` runs it daily via Vercel Cron (`0 6 * * *`).

### Activate

1. Set env in Vercel **Production**:
   - `INDEXNOW_KEY=265a46be97a2ce1f8891dd452d243327` (must equal the
     `public/<KEY>.txt` filename + contents — the code also falls back to this
     literal, so this is belt-and-suspenders).
   - `CRON_SECRET=<openssl rand -hex 32>` (Vercel sends it to the cron
     automatically; required so the endpoint isn't world-triggerable).
2. Redeploy production.
3. Confirm the key file resolves and the ping works:

```bash
curl -i https://www.agentclash.dev/265a46be97a2ce1f8891dd452d243327.txt   # 200 text/plain, body == the key
curl -i -H "authorization: Bearer $CRON_SECRET" https://www.agentclash.dev/api/indexnow   # {ok:true, submitted:N, indexnowStatus:200|202}
```

Domain pre-registration isn't required — the first successful ping with a
resolvable `keyLocation` auto-verifies the domain across participating engines.
IndexNow value is best observed in **Bing Webmaster Tools**, so do section 1
first.

## Notes

- FAQ rich results were discontinued by Google on 2026-05-07; the FAQPage JSON-LD
  the site ships still helps other engines but is no longer a Google rich-result
  lever.
- Tokens are recorded in Vercel env only. If the canonical host ever changes,
  re-verify both consoles and update `INDEXNOW_KEY`/the key file together.
