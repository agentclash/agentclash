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
  return getSeoPagesByPrefix("/industries").map((entry) => ({
    slug: entry.path.replace("/industries/", ""),
  }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const config = getSeoPageByPath(`/industries/${slug}`);
  if (!config) {
    return {};
  }
  return createSeoPageMetadata(config);
}

export default async function IndustrySeoPage({ params }: Props) {
  const { slug } = await params;
  const config = getSeoPageByPath(`/industries/${slug}`);
  if (!config) notFound();
  return <SeoLandingPage config={config} />;
}
