import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";
import { SEO_COLLECTION_INTROS } from "@/lib/seo-pages/collections";

const PAGE_PATH = "/glossary";
const PAGE_TITLE = "Agent Evaluation Glossary - AgentClash";
const PAGE_DESCRIPTION =
  "Definitions for agent evaluation, challenge packs, release gates, and other terms teams use when gating AI agents on real tasks.";

export const metadata = createSeoCollectionMetadata({
  path: PAGE_PATH,
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
});

export default function GlossaryIndexPage() {
  return (
    <SeoCollectionPage
      path={PAGE_PATH}
      title={PAGE_TITLE}
      description={PAGE_DESCRIPTION}
      eyebrow="Glossary"
      h1="Agent evaluation glossary"
      intro={SEO_COLLECTION_INTROS[PAGE_PATH]}
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-glossary-index-schema"
    />
  );
}
