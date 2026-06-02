import sitemap from "@/app/sitemap";
import { DOCS_ORIGIN } from "@/lib/docs";

// IndexNow integration. IndexNow instantly notifies participating search engines
// (Bing, Yandex, Naver, Seznam.cz, Yep — NOT Google, which declined to adopt it)
// that URLs changed. Because Bing feeds Microsoft Copilot, faster Bing indexing
// also improves Copilot answer freshness.
//
// The key is NOT a secret: it only proves domain control by matching the file
// hosted at https://<host>/<KEY>.txt. It is committed (web/public/<KEY>.txt) and
// kept as a fallback here so a misconfigured env never silently no-ops the ping.
export const KEY =
  process.env.INDEXNOW_KEY ?? "265a46be97a2ce1f8891dd452d243327";

// Single source of truth for the host: derive from DOCS_ORIGIN so the IndexNow
// host + keyLocation always match the URLs we submit (engines reject a mismatch).
export const HOST = new URL(DOCS_ORIGIN).host;
export const KEY_LOCATION = `${DOCS_ORIGIN}/${KEY}.txt`;

// Protocol-generic submission endpoint; one POST fans out to all participating
// engines. IndexNow accepts up to 10,000 URLs per request.
const INDEXNOW_ENDPOINT = "https://api.indexnow.org/IndexNow";

// The canonical public URLs to (re)submit — reuse the sitemap so this never
// drifts from what we actually publish. sitemap() already returns absolute URLs.
export function buildUrlList(): string[] {
  return sitemap().map((entry) => entry.url);
}

export type IndexNowResult = { status: number; body: string };

// POST the URL list to IndexNow. Never throws on a non-2xx response — returns the
// upstream status so callers can surface it (e.g. 403 = key/keyLocation mismatch).
export async function submitIndexNow(
  urlList: string[],
): Promise<IndexNowResult> {
  const res = await fetch(INDEXNOW_ENDPOINT, {
    method: "POST",
    headers: { "Content-Type": "application/json; charset=utf-8" },
    body: JSON.stringify({
      host: HOST,
      key: KEY,
      keyLocation: KEY_LOCATION,
      urlList,
    }),
  });
  const body = await res.text();
  return { status: res.status, body };
}
