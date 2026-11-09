import { FactualPage, factualMetadata } from "@/components/marketing/factual-page";
import { FACTUAL_PAGES } from "@/lib/factual-pages";

export const metadata = factualMetadata(FACTUAL_PAGES.methodology);

export default function MethodologyPage() {
  return <FactualPage page={FACTUAL_PAGES.methodology} />;
}
