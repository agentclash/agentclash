import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SeoLandingPage } from "@/components/marketing/seo-landing-page";
import {
  createSeoPageMetadata,
  getSeoPageByPath,
  getSeoPagesByPrefix,
} from "@/lib/seo-pages";

type Props = {
  params: Promise<{ slug: string }>;
};

export const dynamicParams = false;

export function generateStaticParams() {
  return getSeoPagesByPrefix("/use-cases").map((entry) => ({
    slug: entry.path.replace("/use-cases/", ""),
  }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const config = getSeoPageByPath(`/use-cases/${slug}`);
  if (!config) {
    return {};
  }
  return createSeoPageMetadata(config);
}

export default async function UseCaseSeoPage({ params }: Props) {
  const { slug } = await params;
  const config = getSeoPageByPath(`/use-cases/${slug}`);
  if (!config) notFound();
  return <SeoLandingPage config={config} />;
}
