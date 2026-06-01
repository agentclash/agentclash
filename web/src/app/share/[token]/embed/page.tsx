import { notFound } from "next/navigation";

import { PublicScorecardEmbed } from "@/components/share/public-scorecard-embed";
import { canRenderScorecardEmbed } from "@/components/share/public-scorecard-embed-utils";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { PublicShareResponse } from "@/lib/api/types";

export const dynamic = "force-dynamic";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const share = await loadPublicShare(token).catch(() => null);
  return {
    title: share ? `${embedTitle(share)} embed` : "AgentClash scorecard embed",
    robots: { index: false, follow: false },
  };
}

export default async function PublicScorecardEmbedPage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const share = await loadPublicShare(token);
  const resource = share.resource as Record<string, unknown>;

  if (!canRenderScorecardEmbed(resource)) {
    notFound();
  }

  return (
    <main className="min-h-screen bg-transparent">
      <PublicScorecardEmbed resource={resource} />
    </main>
  );
}

async function loadPublicShare(token: string): Promise<PublicShareResponse> {
  const api = createApiClient();
  try {
    return await api.get<PublicShareResponse>(
      `/public/shares/${encodeURIComponent(token)}`,
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }
}

function embedTitle(share: PublicShareResponse) {
  if (share.resource.type === "run_scorecard") {
    const run = share.resource.run as { name?: string } | undefined;
    return run?.name ?? "Run scorecard";
  }
  if (share.resource.type === "run_agent_scorecard") {
    const agent = share.resource.run_agent as { label?: string } | undefined;
    return agent?.label ?? "Agent scorecard";
  }
  return "AgentClash scorecard";
}
