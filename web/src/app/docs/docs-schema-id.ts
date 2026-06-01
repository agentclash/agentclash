export function docsSchemaId(href: string) {
  const pathname = href
    .replace(/^https?:\/\/[^/]+/, "")
    .replace(/\/+$/, "");
  const slug = pathname
    .replace(/^\/docs(?:\/|$)/, "")
    .replace(/^\/+|\/+$/g, "")
    .replace(/\//g, "-");

  return `agentclash-docs-${slug || "home"}-schema`;
}
