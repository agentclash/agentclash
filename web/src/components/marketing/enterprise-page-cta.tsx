"use client";

import { CTAStrip } from "./cta-strip";

const ENTERPRISE_EMAIL = "hello@agentclash.dev";

export function EnterprisePageCTA({ className = "" }: { className?: string }) {
  return (
    <div className={className}>
      <CTAStrip
        variant="demo-first"
        demoLabel="Book eval architecture review"
        secondaryHref={`mailto:${ENTERPRISE_EMAIL}?subject=AgentClash%20enterprise%20eval`}
        secondaryLabel="Email hello@agentclash.dev"
        showGithub={false}
      />
    </div>
  );
}
