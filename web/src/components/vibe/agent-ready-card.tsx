"use client";

import { ArrowRight, MessageSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Artifact } from "@/lib/vibe";

export function AgentReadyCard({
  artifact,
  count,
  busy,
  onChat,
  onTests,
  onInstructions,
}: {
  artifact: Artifact;
  count?: number;
  busy: boolean;
  onChat: () => void;
  onTests: () => void;
  onInstructions: () => void;
}) {
  const plan = artifact.kind === "test_plan";
  return (
    <section
      aria-label="Your agent"
      className="space-y-5 rounded-2xl border border-builder-border bg-builder-surface p-6 sm:p-7"
    >
      <div>
        <p className="text-sm font-medium text-builder-fg-muted">
          {plan
            ? "Your test plan is ready"
            : artifact.parent_id
              ? "Your changes are ready"
              : "Your agent is ready"}
        </p>
        <h1 className="mt-2 text-2xl font-semibold tracking-tight">
          {artifact.title}
        </h1>
        {(artifact.summary || artifact.test_plan?.objective) && (
          <p className="mt-3 text-sm leading-7 text-builder-fg-muted">
            {artifact.summary || artifact.test_plan?.objective}
          </p>
        )}
      </div>
      <div className="border-t border-builder-border pt-5">
        <p className="text-sm font-medium">
          {plan
            ? "Next: review the situations you want to test"
            : "Next: have a conversation with your agent"}
        </p>
        <p className="mt-2 text-sm leading-6 text-builder-fg-muted">
          {plan
            ? "Your existing agent is not connected. Export this plan to run tests in your own environment."
            : "Give it a real message or task and see how it replies. You can ask follow-up questions."}
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          {!plan && (
            <Button disabled={busy} onClick={onChat}>
              <MessageSquare size={16} /> Chat with your agent{" "}
              <ArrowRight size={15} />
            </Button>
          )}
          <Button variant={plan ? "default" : "outline"} onClick={onTests}>
            {plan
              ? "Review test plan"
              : `Review${count ? ` ${count}` : ""} test cases`}
          </Button>
        </div>
        {!plan && (
          <p className="mt-3 text-xs leading-5 text-builder-fg-muted">
            Test cases are example messages with an expected response. Running
            them shows what your agent gets right and where it needs work.
          </p>
        )}
      </div>
      {!plan && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-t border-builder-border pt-4">
          <span className="text-xs text-builder-fg-muted">
            This agent replies in text. It cannot take actions outside this
            page.
          </span>
          <Button variant="ghost" size="sm" onClick={onInstructions}>
            Edit instructions
          </Button>
        </div>
      )}
    </section>
  );
}
