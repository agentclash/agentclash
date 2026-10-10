/**
 * Curated public replay linked from the landing page and (separately) the
 * benchmark report frontmatter, as zero-signup proof of the product.
 * See: https://github.com/agentclash/agentclash/issues/1242
 *
 * This is deliberately `null` until a maintainer completes the *manual*,
 * non-code half of #1242 -- there is no API or CLI path to mint a share
 * token, so this cannot be filled in from a PR alone:
 *
 *   1. Run a real eval (the Expression Evaluator Arena benchmark run is the
 *      obvious candidate -- it already has a published narrative) and open
 *      the winning agent's replay.
 *   2. Click Share. Leave the expiry unset (a UI-minted share never
 *      expires) and decide once whether to mint with
 *      `search_indexing: true` -- doing so also requires a robots.ts
 *      carve-out, since "/share/" is currently disallowed for every
 *      crawler. See the "indexing" acceptance criterion on #1242.
 *   3. Set CURATED_REPLAY_SHARE_URL below to that share URL. Afterwards,
 *      never revoke or re-share the same replay:
 *      public_share_links_active_resource_unique means doing so mints a
 *      *new* token, silently breaking this link.
 *   4. Separately, set `runShareUrl` in
 *      web/content/benchmarks/gpt-generations-expression-evaluator.mdx to
 *      the same URL -- that page already renders a link the moment the
 *      field is non-empty, no code change needed there.
 *
 * Every consumer must treat this as possibly null and render nothing (not
 * a broken link) until it is set -- see the usage in landing.tsx.
 */
export const CURATED_REPLAY_SHARE_URL: string | null = null;
