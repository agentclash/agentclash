import path from "node:path";
import { fromMarkdown, type Extension, type Handle } from "mdast-util-from-markdown";
import { mdxJsxFromMarkdown } from "mdast-util-mdx-jsx";
import { mdxJsx } from "micromark-extension-mdx-jsx";

export type MarkdownDestination = {
  href: string;
  start: number;
  end: number;
  line: number;
  syntax: "markdown" | "jsx";
};

/** Read actual destinations, using parser token offsets instead of matching link-shaped text. */
export function inspectMarkdownDestinations(content: string): MarkdownDestination[] {
  const destinations: MarkdownDestination[] = [];
  const definitions: Array<{ destination: MarkdownDestination; node: { identifier: string } }> = [];
  const jsx = mdxJsxFromMarkdown();
  const attributes: Array<{ destination: Omit<MarkdownDestination, "href">; value: () => unknown }> = [];
  const destination: Handle = function (token) {
    const href = this.resume();
    const node = this.stack[this.stack.length - 1];
    if (node.type !== "link" && node.type !== "image" && node.type !== "definition") return;
    node.url = href;
    // Images are assets, even if their URL happens to look like a docs page.
    if (node.type === "image") return;
    const entry: MarkdownDestination = { href, start: token.start.offset, end: token.end.offset, line: token.start.line, syntax: "markdown" };
    if (node.type === "definition") definitions.push({ destination: entry, node });
    else destinations.push(entry);
  };
  const attribute: Handle = function (token) {
    jsx.exit![token.type].call(this, token);
    const tag = this.data.mdxJsxTag;
    const attr = tag?.attributes[tag.attributes.length - 1];
    if ((tag?.name === "a" || tag?.name === "Link") && attr?.type === "mdxJsxAttribute" && attr.name === "href") {
      attributes.push({
        destination: { start: token.start.offset, end: token.end.offset, line: token.start.line, syntax: "jsx" },
        value: () => attr.value,
      });
    }
  };
  const extension: Extension = {
    exit: {
      resourceDestinationString: destination,
      definitionDestinationString: destination,
      mdxJsxFlowTagAttributeValueLiteralValue: attribute,
      mdxJsxTextTagAttributeValueLiteralValue: attribute,
    },
  };
  const tree = fromMarkdown(content, {
    extensions: [mdxJsx()],
    mdastExtensions: [jsx, extension],
  });
  const imageReferences = new Set<string>();
  function collectImages(node: typeof tree | (typeof tree.children)[number]) {
    if (node.type === "imageReference") imageReferences.add(node.identifier.toUpperCase().replace(/\s+/g, " "));
    if ("children" in node) for (const child of node.children) collectImages(child);
  }
  collectImages(tree);
  // A shared definition must keep its original target for the image.
  for (const { destination, node } of definitions) {
    if (!imageReferences.has(node.identifier.toUpperCase().replace(/\s+/g, " "))) destinations.push(destination);
  }
  for (const { destination, value } of attributes) {
    const href = value();
    if (typeof href === "string") destinations.push({ ...destination, href });
  }
  return destinations.sort((a, b) => a.start - b.start);
}

/** Change destination bytes only; keep titles, code, labels, whitespace and formatting intact. */
export function rewriteMarkdownDestinations(content: string, rewrite: (href: string) => string): string {
  const destinations = inspectMarkdownDestinations(content);
  for (const destination of destinations.reverse()) {
    const href = rewrite(destination.href);
    if (href === destination.href) continue;
    // Escape ampersands so entity-like query and fragment values survive reparsing.
    const encoded = destination.syntax === "jsx"
      ? href.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("'", "&#39;").replaceAll("<", "&lt;")
      : href.replace(/([\\()<>])/g, "\\$1").replaceAll("&", "&amp;");
    content = content.slice(0, destination.start) + encoded + content.slice(destination.end);
  }
  return content;
}

export function isRelativePageDestination(href: string): boolean {
  if (/^(?:[a-z][a-z0-9+.-]*:|[\/#?])/i.test(href)) return false;
  const pathname = href.split(/[?#]/, 1)[0];
  return !path.posix.extname(pathname) || /\.mdx?$/i.test(pathname);
}

/** Resolve relative paths from the source file, including directory index and SKILL.md sources. */
export function resolveDocsDestination(
  href: string,
  sourcePath: string | undefined,
  docsDirectory: string,
  skillsDirectory: string,
  knownPaths: ReadonlySet<string>,
): string | null {
  if (/^(?:[a-z][a-z0-9+.-]*:|\/\/|#|\?)/i.test(href)) return null;
  const split = href.search(/[?#]/);
  const pathname = split === -1 ? href : href.slice(0, split);
  const suffix = split === -1 ? "" : href.slice(split);
  let canonical: string;
  if (pathname.startsWith("/")) {
    canonical = pathname;
  } else {
    if (!sourcePath) return null;
    const target = path.resolve(path.dirname(sourcePath), pathname);
    const docsRelative = path.relative(docsDirectory, target);
    const skillsRelative = path.relative(skillsDirectory, target);
    if (!docsRelative.startsWith("..") && !path.isAbsolute(docsRelative)) {
      canonical = `/docs/${docsRelative.split(path.sep).join("/")}`;
    } else if (!skillsRelative.startsWith("..") && !path.isAbsolute(skillsRelative)) {
      canonical = `/docs/agent-skills/${skillsRelative.split(path.sep).join("/").replace(/(?:^|\/)SKILL\.md$/, "")}`;
    } else return null;
  }
  canonical = canonical.replace(/\.mdx?$/, "").replace(/\/index$/, "").replace(/\/$/, "");
  return knownPaths.has(canonical) ? canonical + suffix : null;
}
