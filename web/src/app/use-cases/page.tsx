import {
  SeoCollectionPage,
  createSeoCollectionMetadata,
} from "@/components/marketing/seo-collection-page";
import { getSeoPagesByPrefix } from "@/lib/seo-pages";

const PAGE_PATH = "/use-cases";
const PAGE_TITLE = "Agent Evaluation Use Cases - AgentClash";
const PAGE_DESCRIPTION =
  "Explore AgentClash use cases for coding, research, and customer support agent evaluation with replay evidence and CI gates.";

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
      intro="Pick the workload that matches your team — coding agents, research agents, or customer support agents — and evaluate them on real tasks with replay and scorecards."
      pages={getSeoPagesByPrefix(PAGE_PATH)}
      schemaId="agentclash-use-cases-index-schema"
    />
  );
}
