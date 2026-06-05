export type SeoPageTier = "S" | "A" | "B";

export type SeoPageWorkflowStep = {
  title: string;
  text: string;
};

export type SeoPageFaqItem = {
  question: string;
  answer: string;
};

export type SeoPageRelatedLink = {
  title: string;
  text: string;
  href: string;
};

export type SeoPageBreadcrumb = {
  name: string;
  url: string;
};

export type SeoPageConfig = {
  path: string;
  tier: SeoPageTier;
  keyword: string;
  intent: string;
  pageTitle: string;
  metaDescription: string;
  socialImageAlt: string;
  eyebrow: string;
  h1: string;
  heroDescription: string;
  proofSectionTitle: string;
  proofSectionDescription: string;
  proofPoints: string[];
  workflowSectionTitle: string;
  workflow: SeoPageWorkflowStep[];
  docsSectionTitle: string;
  docsSectionDescription: string;
  relatedLinks: SeoPageRelatedLink[];
  faqSectionTitle: string;
  faqItems: SeoPageFaqItem[];
  applicationSubCategory: string;
  breadcrumbs: SeoPageBreadcrumb[];
  schemaId: string;
  searchKeywords: string;
  sitemapTitle: string;
  sitemapDescription: string;
};
