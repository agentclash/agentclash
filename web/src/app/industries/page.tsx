import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";
import { SEO_COLLECTION_INTROS } from "@/lib/seo-pages/collections";

const PAGE_PATH = "/industries";
const PAGE_TITLE = "Agent Evaluation by Industry - AgentClash";
const PAGE_DESCRIPTION =
  "Evaluate AI agents in banking, insurance, and government with replay evidence, scorecards, and release gates your reviewers can trust.";

export const metadata = createSeoCollectionMetadata({
  path: PAGE_PATH,
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
});

export default function IndustriesIndexPage() {
  return (
    <SeoCollectionPage
      path={PAGE_PATH}
      title={PAGE_TITLE}
      description={PAGE_DESCRIPTION}
      eyebrow="Industries"
      h1="Agent evaluation for regulated industries"
      intro={SEO_COLLECTION_INTROS[PAGE_PATH]}
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-industries-index-schema"
    />
  );
}
