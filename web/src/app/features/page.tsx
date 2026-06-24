import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";

const PAGE_PATH = "/features";
const PAGE_TITLE = "Agent Evaluation Features - AgentClash";
const PAGE_DESCRIPTION =
  "Explore AgentClash features for agent scorecards, replay evidence, and eval packs that turn real tasks into repeatable evaluations.";

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
      intro="See how AgentClash turns agent runs into reviewable scorecards, replay evidence, and reusable eval packs for CI-ready evaluation."
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-features-index-schema"
    />
  );
}
