import Link from "next/link";
import { ChangelogTimeline } from "@/components/marketing/changelog-timeline";
import { JsonLd, changelogIndexSchema } from "@/components/marketing/json-ld";
import { CHANGELOG_FAQ, getChangelogPeriods } from "@/lib/changelog";
import { changelogMetadata } from "./metadata";

export const metadata = changelogMetadata;

export default function ChangelogPage() {
  const periods = getChangelogPeriods();

  return (
    <>
      <JsonLd
        id="agentclash-changelog-index-schema"
        data={changelogIndexSchema(
          periods.map((period) => ({
            id: period.id,
            label: period.label,
            headline: period.headline,
            endDate: period.endDate,
            entryCount: period.entries.length,
          })),
          [...CHANGELOG_FAQ],
        )}
      />
      <main className="min-h-screen flex flex-col items-center px-6 py-16">
        <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/35">
          Product updates
        </p>
        <h1 className="mt-4 font-[family-name:var(--font-display)] text-3xl sm:text-4xl text-center tracking-[-0.02em] leading-[1.15]">
          Changelog
        </h1>
        <p className="mt-3 max-w-xl text-center text-sm leading-relaxed text-white/35">
          What we shipped in AgentClash, grouped every ten days from the first
          commit on April 15, 2026 through today.
        </p>

        <ChangelogTimeline periods={periods} />

        <Link
          href="/"
          className="mt-12 text-xs text-white/30 hover:text-white/50 transition-colors"
        >
          &larr; Back to AgentClash
        </Link>
      </main>
    </>
  );
}
