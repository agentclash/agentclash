import library from "../../content/resources/ai-agent-eval-library.json";

export type ResourceLibraryItem = {
  slug: string;
  title: string;
  kicker: string;
  description: string;
  audience: string;
  readTime: string;
  file: string;
  sections: Array<{
    title: string;
    items: string[];
  }>;
};

export const RESOURCE_LIBRARY = library.resources as ResourceLibraryItem[];

export const PRIMARY_RESOURCE =
  RESOURCE_LIBRARY.find((resource) => resource.slug === library.primarySlug) ??
  RESOURCE_LIBRARY[0];

export function getResourceBySlug(slug: string) {
  return RESOURCE_LIBRARY.find((resource) => resource.slug === slug) ?? null;
}
