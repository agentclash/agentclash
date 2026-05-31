import { TryCliDemoClient } from "@/components/try-cli/demo-client";

interface Props {
  params: Promise<{ slug: string }>;
}

export default async function TryCliDemoPage({ params }: Props) {
  const { slug } = await params;
  return <TryCliDemoClient slug={slug} />;
}

export async function generateMetadata({ params }: Props) {
  const { slug } = await params;
  return {
    title: `Try ${slug} in browser | AgentClash`,
    description: `Interactive terminal demo for ${slug}. No install required.`,
  };
}
