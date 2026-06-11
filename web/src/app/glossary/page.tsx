import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";

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
      intro="Plain-language definitions for the concepts behind AgentClash: what agent evaluation means, how challenge packs work, and what release gates enforce before you ship."
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-glossary-index-schema"
    />
  );
}
