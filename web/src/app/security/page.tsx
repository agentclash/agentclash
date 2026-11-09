import { FactualPage, factualMetadata } from "@/components/marketing/factual-page";
import { FACTUAL_PAGES } from "@/lib/factual-pages";

export const metadata = factualMetadata(FACTUAL_PAGES.security);

export default function SecurityPage() {
  return <FactualPage page={FACTUAL_PAGES.security} />;
}
