"use client";

import { useState } from "react";
import { readExampleCard } from "@/lib/vibe-presentation";
import { VibeButton } from "./vibe-button";

export function ExampleComparison({ input, expected }: { input: string; expected: string }) {
  return <section aria-label="Illustrative test" className="vibe-example">
    <div className="vibe-example-heading"><span>Example only</span><span>Nothing was run</span></div>
    <div className="vibe-example-columns">
      <div><h3>Task</h3><p>{input}</p></div>
      <div><h3>What a good result looks like</h3><p>{expected}</p></div>
    </div>
  </section>;
}

export function ConversationGuidance({ cards, scope, message }: { cards?: unknown[]; scope?: string; message: string }) {
  const [hidden, setHidden] = useState(false);
  if (!cards || cards.length !== 1) return null;
  const example = readExampleCard(cards[0], scope, message);
  if (!example) return null;
  return <div className="mt-4 space-y-2" data-guidance-id={example.id}>
    {!hidden && <ExampleComparison input={example.input} expected={example.expected} />}
    <VibeButton variant="quiet" className="!px-0" aria-expanded={!hidden} onClick={() => setHidden(!hidden)}>{hidden ? "Show example" : "Hide example"}</VibeButton>
  </div>;
}

export function TestMeaning() {
  return <details className="vibe-test-meaning text-sm vibe-muted">
    <summary>What’s a test?</summary>
    <p className="mt-2">A test is a task for your agent and what a good result should look like. We compare your agent’s reply with that expectation.</p>
  </details>;
}
