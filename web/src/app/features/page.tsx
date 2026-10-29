import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";
import { SEO_COLLECTION_INTROS } from "@/lib/seo-pages/collections";

const PAGE_PATH = "/features";
const PAGE_TITLE = "Agent Evaluation Features - AgentClash";
const PAGE_DESCRIPTION =
  "Explore AgentClash features for agent scorecards, replay evidence, and challenge packs that turn real tasks into repeatable evaluations.";

export const metadata = createSeoCollectionMetadata({
  path: PAGE_PATH,
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
});

export default function FeaturesIndexPage() {
  return (
    <SeoCollectionPage
      path={PAGE_PATH}
      title={PAGE_TITLE}
      description={PAGE_DESCRIPTION}
      eyebrow="Features"
      h1="Agent evaluation features"
      intro={SEO_COLLECTION_INTROS[PAGE_PATH]}
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-features-index-schema"
    />
  );
}
