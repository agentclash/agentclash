import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";
import { SEO_COLLECTION_INTROS } from "@/lib/seo-pages/collections";

const PAGE_PATH = "/use-cases";
const PAGE_TITLE = "Agent Evaluation Use Cases - AgentClash";
const PAGE_DESCRIPTION =
  "Explore AgentClash use cases for coding, research, and customer support agent evaluation, plus industry playbooks for regulated teams.";

export const metadata = createSeoCollectionMetadata({
  path: PAGE_PATH,
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
});

export default function UseCasesIndexPage() {
  return (
    <SeoCollectionPage
      path={PAGE_PATH}
      title={PAGE_TITLE}
      description={PAGE_DESCRIPTION}
      eyebrow="Use cases"
      h1="Agent evaluation use cases"
      intro={SEO_COLLECTION_INTROS[PAGE_PATH]}
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      secondarySections={[
        {
          title: "Industries",
          intro:
            "Regulated buyers in banking, insurance, and government can start from vertical playbooks that link to the same replay and gate workflow.",
          pages: getSeoPagesByPrefix("/industries"),
        },
      ]}
      schemaId="agentclash-use-cases-index-schema"
    />
  );
}
