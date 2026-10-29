import fs from "node:fs";
import path from "node:path";
import { fromMarkdown } from "mdast-util-from-markdown";
import { mdxJsxFromMarkdown } from "mdast-util-mdx-jsx";
import { mdxJsx } from "micromark-extension-mdx-jsx";

const root = path.resolve(process.argv[2] ?? "content/docs");
let errors = 0;
function check(href, file, line) {
  if (/^(?:[a-z][a-z0-9+.-]*:|[\/#?])/i.test(href)) return;
  const pathname = href.split(/[?#]/, 1)[0];
  if (path.posix.extname(pathname) && !/\.mdx?$/i.test(pathname)) return;
  console.error(`${file}:${line}: relative page link ${href}; use a root-relative /docs/ URL`);
  errors++;
}
function visit(node, file, imageReferences, definitions) {
  if (node.type === "link" || (node.type === "definition" && !imageReferences.has(normalize(node.identifier)))) check(node.url, file, node.position.start.line);
  if (node.type === "linkReference" && imageReferences.has(normalize(node.identifier))) {
    const definition = definitions.get(normalize(node.identifier));
    if (definition) check(definition.url, file, node.position.start.line);
  }
  if ((node.type === "mdxJsxFlowElement" || node.type === "mdxJsxTextElement") && (node.name === "a" || node.name === "Link")) {
    for (const attr of node.attributes) {
      if (attr.type === "mdxJsxAttribute" && attr.name === "href" && typeof attr.value === "string") check(attr.value, file, node.position.start.line);
    }
  }
  if (node.children) for (const child of node.children) visit(child, file, imageReferences, definitions);
}
function normalize(identifier) { return identifier.toUpperCase().replace(/\s+/g, " "); }
function collectImages(node, imageReferences, definitions) {
  if (node.type === "imageReference") imageReferences.add(normalize(node.identifier));
  if (node.type === "definition" && !definitions.has(normalize(node.identifier))) definitions.set(normalize(node.identifier), node);
  if (node.children) for (const child of node.children) collectImages(child, imageReferences, definitions);
}
function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(file);
    else if (/\.mdx?$/.test(entry.name)) {
      const tree = fromMarkdown(fs.readFileSync(file, "utf8"), { extensions: [mdxJsx()], mdastExtensions: [mdxJsxFromMarkdown()] });
      const imageReferences = new Set();
      const definitions = new Map();
      collectImages(tree, imageReferences, definitions);
      visit(tree, file, imageReferences, definitions);
    }
  }
}
walk(root);
if (errors) process.exitCode = 1;
else console.log(`Docs links OK: ${root}`);
