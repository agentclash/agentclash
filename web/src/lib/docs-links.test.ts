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

  it("preserves entity-like query and fragment values when reparsing exported links", () => {
    const doc = getDocBySlug([])!;
    const markdown = renderDocMarkdown({ ...doc, content: "[Page](/docs?flag=1&amp;copy;#part&amp;trade;)" }, origin);
    expect(markdownDestinations(markdown)).toContain(`${origin}/md/docs?flag=1&copy;#part&trade;`);
  });

  it.each([" ", "&#32;", "&#9;", "&#10;", "&#13;"])("preserves decoded whitespace %s in inline and shared destinations", (entity) => {
    const doc = getDocBySlug([])!;
    const decoded = entity === " " ? " " : String.fromCharCode(Number(entity.slice(2, -1)));
    const content = `[Inline](</docs?q=a${entity}b>) [A *formatted* page][shared] ![Image][shared]\n\n[shared]: </docs?q=a${entity}b>`;
    const tree = fromMarkdown(renderDocMarkdown({ ...doc, content }, origin));
    const paragraph = tree.children.find((node) => node.type === "paragraph" && node.children.some((child) => child.type === "imageReference"));
    if (paragraph?.type !== "paragraph") throw new Error("Missing paragraph");
    const links = paragraph.children.filter((node) => node.type === "link");
    expect(links.map((node) => node.url)).toEqual(Array(2).fill(`${origin}/md/docs?q=a${decoded}b`));
    expect(links[1].children.some((node) => node.type === "emphasis")).toBe(true);
    expect(paragraph.children.filter((node) => node.type === "imageReference")).toHaveLength(1);
    expect(tree.children.find((node) => node.type === "definition")?.url).toBe(`/docs?q=a${decoded}b`);
  });

  it("exports shared reference links while preserving image targets and formatted labels", () => {
    const doc = getDocBySlug([])!;
    const content = ["![Image][asset] ![Shared image][shared] [A *formatted* page][shared] [shared][] [shared]", "[asset]: /docs", '[shared]: /docs?x=1&amp;copy;#part "A title"'].join("\n\n");
    const markdown = renderDocMarkdown({ ...doc, content }, origin);
    const tree = fromMarkdown(markdown);
    const paragraph = tree.children.find((node) => node.type === "paragraph" && node.children.some((child) => child.type === "imageReference"));
    if (paragraph?.type !== "paragraph") throw new Error("Missing paragraph");
    const link = paragraph.children.find((node) => node.type === "link");
    expect(link?.type === "link" && link.url).toBe(`${origin}/md/docs?x=1&copy;#part`);
    expect(link?.type === "link" && link.title).toBe("A title");
    expect(link?.type === "link" && link.children.some((child) => child.type === "emphasis")).toBe(true);
    expect(paragraph.children.filter((node) => node.type === "link").map((node) => node.url)).toEqual(Array(3).fill(`${origin}/md/docs?x=1&copy;#part`));
    expect(tree.children.filter((node) => node.type === "definition").map((node) => node.url)).toEqual(["/docs", "/docs?x=1&copy;#part"]);
    expect(paragraph.children.filter((node) => node.type === "imageReference")).toHaveLength(2);
  });

  it("preserves backtick runs and brackets inside a shared reference label", () => {
    const doc = getDocBySlug([])!;
    const content = "[a ``x````` ] y``][shared]\n\n![Image][shared]\n\n[shared]: /docs";
    const tree = fromMarkdown(renderDocMarkdown({ ...doc, content }, origin));
    const paragraph = tree.children.find((node) => node.type === "paragraph" && node.children.some((child) => child.type === "link"));
    const link = paragraph?.type === "paragraph" ? paragraph.children.find((node) => node.type === "link") : undefined;
    expect(link?.type === "link" && link.url).toBe(`${origin}/md/docs`);
    expect(link?.type === "link" && link.children.find((node) => node.type === "inlineCode")?.value).toBe("x````` ] y");
    expect(tree.children.find((node) => node.type === "definition")?.url).toBe("/docs");
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

it("uses authored dates and leaves unknown generated documentation dates unset", () => {
  expect(getDocBySlug([])?.dateModified).toBe("2026-08-09");
  expect(getDocBySlug(["agent-skills"])?.dateModified).toBeUndefined();
});
