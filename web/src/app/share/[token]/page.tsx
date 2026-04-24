import Link from "next/link";
import { notFound } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { PublicShareRenderer } from "@/components/share/public-share-renderers";
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
  if (!share) return { title: "Shared AgentClash artifact" };

  return {
    title: publicTitle(share),
    robots: share.share.search_indexing
      ? { index: true, follow: true }
      : { index: false, follow: false },
  };
}

export default async function PublicSharePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const share = await loadPublicShare(token);

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-8 sm:px-6 lg:px-8">
        <header className="flex flex-wrap items-start justify-between gap-4 border-b border-border pb-5">
          <div>
            <Link
              href="/"
              className="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground hover:text-foreground"
            >
              AgentClash
            </Link>
            <h1 className="mt-2 text-2xl font-semibold tracking-tight">
              {publicTitle(share)}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Public read-only share
            </p>
          </div>
          <Badge variant="outline">{share.resource.type}</Badge>
        </header>

        <PublicResourceSummary share={share} />
        <PublicShareRenderer resource={share.resource as Record<string, unknown>} />
      </div>
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

function publicTitle(share: PublicShareResponse) {
  const resource = share.resource;
  if (resource.type === "challenge_pack_version") {
    const pack = resource.pack as { name?: string } | undefined;
    const version = resource.version as { version_number?: number } | undefined;
    return `${pack?.name ?? "Challenge pack"} v${version?.version_number ?? ""}`.trim();
  }
  if (resource.type === "run_scorecard") {
    const run = resource.run as { name?: string } | undefined;
    return `${run?.name ?? "Run"} scorecard`;
  }
  if (resource.type === "run_agent_scorecard") {
    const agent = resource.run_agent as { label?: string } | undefined;
    return `${agent?.label ?? "Agent"} scorecard`;
  }
  if (resource.type === "run_agent_replay") {
    const agent = resource.run_agent as { label?: string } | undefined;
    return `${agent?.label ?? "Agent"} replay`;
  }
  return "Shared AgentClash artifact";
}

function PublicResourceSummary({ share }: { share: PublicShareResponse }) {
  const created = new Date(share.share.created_at).toLocaleString();
  return (
    <section className="grid gap-3 sm:grid-cols-3">
      <SummaryItem label="Shared" value={created} />
      <SummaryItem label="Views" value={String(share.share.view_count)} />
      <SummaryItem
        label="Indexing"
        value={share.share.search_indexing ? "Allowed" : "Noindex"}
      />
    </section>
  );
}

function SummaryItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card px-4 py-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-sm font-medium">{value}</div>
    </div>
  );
}
