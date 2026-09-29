"use client";

import { useState } from "react";
import { AgentReply } from "./safe-markdown";
import { VibeButton } from "./vibe-button";
import { TaskSources } from "./task-sources";
import type { InputBinding } from "@/lib/vibe-inputs";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";

export function TaskOutput({ text, sessionID, materials = [] }: { text: string; sessionID?: string; materials?: InputBinding[] }) {
  const [notice, setNotice] = useState("");
  let json = false; try { JSON.parse(text); json = true; } catch { /* Plain text output. */ }
  function download(asJSON = false) {
    if (sessionID) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXPORT_REQUESTED, { session_id: sessionID, format: asJSON ? "json" : "markdown" });
    const url = URL.createObjectURL(new Blob([text], { type: asJSON ? "application/json" : "text/markdown;charset=utf-8" }));
    const link = document.createElement("a"); link.href = url; link.download = asJSON ? "agent-output.json" : "agent-output.md"; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  return <section aria-label="Your agent’s output"><AgentReply>{text}</AgentReply><div className="flex flex-wrap gap-2 mt-3">
    <VibeButton variant="quiet" onClick={() => void navigator.clipboard.writeText(text).then(() => setNotice("Copied")).catch(() => setNotice("Select the output and copy it."))}>Copy output</VibeButton>
    <VibeButton variant="quiet" onClick={() => download()}>Download output</VibeButton>
    {json && <VibeButton variant="quiet" onClick={() => download(true)}>Download JSON</VibeButton>}
  </div><span role="status" className="text-xs vibe-muted">{notice}</span>{sessionID && materials.length > 0 && <TaskSources sessionID={sessionID} bindings={materials} />}</section>;
}
