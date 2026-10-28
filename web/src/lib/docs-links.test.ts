import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { fromMarkdown } from "mdast-util-from-markdown";
import { getAllDocSlugs, getDocBySlug, renderDocMarkdown } from "./docs";
import { getPostBySlug } from "./blog";

const origin = "https://example.test";

function markdownDestinations(content: string): string[] {
  const destinations: string[] = [];
  function walk(node: ReturnType<typeof fromMarkdown> | ReturnType<typeof fromMarkdown>["children"][number]) {
    if ((node.type === "link" || node.type === "definition") && "url" in node) destinations.push(node.url);
    if ("children" in node) for (const child of node.children) walk(child);
  }
  walk(fromMarkdown(content));
  return destinations;
}

describe("docs link destinations", () => {
  it.each([
    [["challenge-packs"], "bundle-yaml-reference", "/docs/challenge-packs/bundle-yaml-reference"],
    [["challenge-packs", "evaluation-spec-reference"], "llm-judges.mdx?mode=pair#budgets", "/docs/challenge-packs/llm-judges?mode=pair#budgets"],
    [[], "guides/interpret-results", "/docs/guides/interpret-results"],
    [["agent-skills", "agentclash-hub"], "../agentclash-quickstart/SKILL.md#setup", "/docs/agent-skills/agentclash-quickstart#setup"],
  ])("resolves source-relative links for %j", (slug, href, expected) => {
    const doc = getDocBySlug(slug)!;
    const markdown = renderDocMarkdown({ ...doc, content: `[Read](${href})` }, origin);
    expect(markdownDestinations(markdown)).toContain(`${origin}/md${expected}`);
  });

  it("rewrites parsed inline, reference and MDX anchor destinations while preserving surrounding source", () => {
    const content = [
      '[Inline](/docs?view=all#start "Title")',
      '[Reference][guide]',
      '[guide]: <../guides/interpret-results.mdx?view=all#scores> "Reference title"',
      '<a className="guide" href="../guides/interpret-results?view=all&amp;mode=full#scores">Guide</a>',
      '<a href={dynamicHref}>Dynamic</a>',
      '[Asset](../images/example.png) ![Image](/docs/example.png)',
      '[Unknown](/docs/does-not-exist)',
      '[External](https://other.test/docs/guide) [Mail](mailto:help@example.test) [Anchor](#local)',
      '`[Code](/docs/guides/interpret-results)`',
      '```md',
      '[Fenced](/docs/guides/interpret-results)',
      '<Callout type="note">Leave literal code alone</Callout>',
      '```',
    ].join("\n\n");
    const doc = getDocBySlug(["challenge-packs", "evaluation-spec-reference"])!;
    const markdown = renderDocMarkdown({ ...doc, content }, origin);
    const expected = content
      .replace('/docs?view=all#start', `${origin}/md/docs?view=all#start`)
      .replace('../guides/interpret-results.mdx?view=all#scores', `${origin}/md/docs/guides/interpret-results?view=all#scores`)
      .replace('../guides/interpret-results?view=all&amp;mode=full#scores', `${origin}/md/docs/guides/interpret-results?view=all&amp;mode=full#scores`);
    expect(markdown.endsWith(expected)).toBe(true);
  });

  it("keeps the generated SKILL.md payload intact including literal docs URLs", () => {
    const slug = ["agent-skills", "agentclash-hub"];
    const raw = fs.readFileSync(path.join(process.cwd(), "content", ...slug, "SKILL.md"), "utf8").trim();
    const markdown = renderDocMarkdown(getDocBySlug(slug)!, origin);
    expect(markdown).toContain(`\n${raw}\n`);
  });

  it("authors docs page destinations as canonical root-relative URLs that resolve", () => {
    for (const slug of getAllDocSlugs().filter((slug) => slug[0] !== "agent-skills")) {
      const doc = getDocBySlug(slug)!;
      for (const href of markdownDestinations(doc.content)) {
        if (/^(?:[a-z][a-z0-9+.-]*:|\/\/|#|\?)/i.test(href)) continue;
        if (/\.[a-z0-9]+(?:[?#]|$)/i.test(href) && !/\.mdx?(?:[?#]|$)/i.test(href)) continue;
        expect(href, `${doc.href}: ${href}`).toMatch(/^\//);
        const url = new URL(href, origin);
        if (url.pathname === "/docs" || url.pathname.startsWith("/docs/")) {
          expect(getDocBySlug(url.pathname.slice(6).split("/").filter(Boolean)), `${doc.href}: ${href}`).not.toBeNull();
        }
      }
    }
  });

  it("exports actual docs destinations that resolve and keeps the blog RSS link on the public feed", () => {
    const doc = getDocBySlug(["challenge-packs"])!;
    const destinations = markdownDestinations(renderDocMarkdown(doc, origin));
    expect(destinations).toContain(`${origin}/md/docs/challenge-packs/bundle-yaml-reference`);
    for (const href of destinations) {
      const url = new URL(href, origin);
      if (url.origin === origin && url.pathname.startsWith("/md/docs")) {
        expect(getDocBySlug(url.pathname.slice(9).split("/").filter(Boolean)), href).not.toBeNull();
      }
    }
    const post = getPostBySlug("product-updates-june-2026")!;
    expect(markdownDestinations(post.content)).toContain("/feed.xml");
  });
});
