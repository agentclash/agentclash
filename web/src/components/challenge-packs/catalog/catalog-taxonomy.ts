import type { CatalogCategory } from "../lib/types";

export interface CatalogCategoryMeta {
  key: CatalogCategory;
  label: string;
  description: string;
}

/**
 * Category taxonomy for the library gallery, kept as data so the gallery groups
 * packs purely off the `category` field. Adding a category is one entry here
 * plus a backend enum value — no per-pack frontend code.
 */
export const CATALOG_CATEGORIES: readonly CatalogCategoryMeta[] = [
  {
    key: "enterprise",
    label: "Enterprise use cases",
    description: "Production agent + LLM tasks enterprises evaluate today.",
  },
  {
    key: "agent_capability",
    label: "Agent capabilities",
    description: "Core agent skills: tool use, structured output, reasoning.",
  },
  {
    key: "safety",
    label: "Safety & security",
    description: "Prompt-injection, jailbreak, and refusal-calibration suites.",
  },
];

export const UNCATEGORIZED_LABEL = "Other";
