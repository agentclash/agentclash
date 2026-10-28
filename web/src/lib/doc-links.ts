import path from "node:path";
import { fromMarkdown, type Extension, type Handle } from "mdast-util-from-markdown";
import { mdxJsxFromMarkdown } from "mdast-util-mdx-jsx";
import { mdxJsx } from "micromark-extension-mdx-jsx";

export type MarkdownDestination = {
  href: string;
  start: number;
  end: number;
  line: number;
  syntax: "markdown" | "jsx" | "reference";
  title?: string | null;
};

function normalizedIdentifier(identifier: string) {
  return identifier.toUpperCase().replace(/\s+/g, " ");
}

// Find the end of the visible label, respecting nested brackets, escapes and code spans.
function labelEnd(content: string, start: number, end: number): number | undefined {
  let depth = 0;
  for (let i = start; i < end; i++) {
    if (content[i] === "\\") { i++; continue; }
    if (content[i] === "`") {
      let j = i;
      while (content[j] === "`") j++;
      const closing = content.indexOf(content.slice(i, j), j);
      if (closing !== -1 && closing < end) { i = closing + (j - i) - 1; continue; }
      i = j - 1;
      continue;
    }
    if (content[i] === "[") depth++;
    if (content[i] === "]" && --depth === 0) return i + 1;
  }
  return undefined;
}

/** Read actual destinations, using parser token offsets instead of matching link-shaped text. */
export function inspectMarkdownDestinations(content: string): MarkdownDestination[] {
  const destinations: MarkdownDestination[] = [];
  const definitions: Array<{ destination: MarkdownDestination; node: { identifier: string; title?: string | null } }> = [];
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
  const linkReferences: Array<{ identifier: string; start: number; end: number; line: number }> = [];
  function collectImages(node: typeof tree | (typeof tree.children)[number]) {
    if (node.type === "imageReference") imageReferences.add(normalizedIdentifier(node.identifier));
    if (node.type === "linkReference" && node.position?.start.offset !== undefined && node.position.end.offset !== undefined) {
      linkReferences.push({ identifier: normalizedIdentifier(node.identifier), start: node.position.start.offset, end: node.position.end.offset, line: node.position.start.line });
    }
    if ("children" in node) for (const child of node.children) collectImages(child);
  }
  collectImages(tree);
  const sharedDefinitions = new Map<string, typeof definitions[number]>();
  for (const { destination, node } of definitions) {
    const identifier = normalizedIdentifier(node.identifier);
    if (!imageReferences.has(identifier)) destinations.push(destination);
    else if (!sharedDefinitions.has(identifier)) sharedDefinitions.set(identifier, { destination, node });
  }
  for (const reference of linkReferences) {
    const definition = sharedDefinitions.get(reference.identifier);
    if (!definition) continue;
    const start = labelEnd(content, reference.start, reference.end);
    if (start !== undefined) destinations.push({ href: definition.destination.href, start, end: reference.end, line: reference.line, syntax: "reference", title: definition.node.title });
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
    const replacement = destination.syntax === "reference"
      ? `(${encoded}${destination.title == null ? "" : ` "${destination.title.replaceAll("\\", "\\\\").replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("\n", "&#10;").replaceAll("\r", "&#13;")}"`})`
      : encoded;
    content = content.slice(0, destination.start) + replacement + content.slice(destination.end);
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
