import type { SeoPageTier } from "./types";

export function seoPageSitemapPriority(tier: SeoPageTier): number {
  switch (tier) {
    case "S":
      return 0.84;
    case "A":
      return 0.8;
    case "B":
      return 0.76;
    default:
      return 0.75;
  }
}
