import { ChangelogIndexList } from "@/components/marketing/changelog/changelog-index-list";
import { ChangelogShell } from "@/components/marketing/changelog/changelog-shell";
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
      <ChangelogShell backHref="/" backLabel="Back to AgentClash">
        <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-white/35">
          Release notes
        </p>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-white sm:text-3xl">
          Changelog
        </h1>
        <p className="mt-3 max-w-xl text-sm leading-relaxed text-white/45">
          Product updates grouped in ten-day windows. Select a release for the
          full breakdown and merged pull requests.
        </p>

        <ChangelogIndexList periods={periods} />
      </ChangelogShell>
    </>
  );
}
