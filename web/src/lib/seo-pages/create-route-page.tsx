import { SeoLandingPage } from "@/components/marketing/seo-landing-page";
import { createSeoPageMetadata, getSeoPageByPath } from "./index";

export function createSeoRoutePage(path: string) {
  const config = getSeoPageByPath(path);
  if (!config) {
    throw new Error(`Missing SEO page config for ${path}`);
  }

  return {
    metadata: createSeoPageMetadata(config),
    Page: function SeoRoutePage() {
      return <SeoLandingPage config={config} />;
    },
  };
}
