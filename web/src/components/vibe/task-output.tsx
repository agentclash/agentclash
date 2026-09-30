"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { AgentReply } from "./safe-markdown";
import { VibeButton } from "./vibe-button";
import { TaskSources } from "./task-sources";
import type { InputBinding } from "@/lib/vibe-inputs";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { jsonOutput, outputDownload, type OutputFormat } from "@/lib/vibe-output-format";

export function TaskOutput({ text, sessionID, materials = [], brief = false }: { text: string; sessionID?: string; materials?: InputBinding[]; brief?: boolean }) {
  const [notice, setNotice] = useState("");
  const [expanded, setExpanded] = useState(false);
  const [long, setLong] = useState(false);
  const content = useRef<HTMLDivElement>(null);
  const hasJSON = jsonOutput(text) !== undefined;
  // Measure rendered content: a short table or a narrow phone can be tall too.
  // Keep the original DOM and bytes intact rather than cutting Markdown/JSON.
  useLayoutEffect(() => {
    const element = content.current;
    if (!element) return;
    const measure = () => setLong(element.scrollHeight > 24 * (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16));
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [text]);
  function download(format: OutputFormat) {
    try {
      const file = outputDownload(text, format);
      if (sessionID) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXPORT_REQUESTED, { session_id: sessionID, format });
      const url = URL.createObjectURL(new Blob([file.contents], { type: file.type }));
      const link = document.createElement("a"); link.href = url; link.download = brief ? file.filename.replace("agent-output", "project-brief") : file.filename; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
      setNotice(`${format === "json" ? "JSON" : "Markdown"} download started`);
    } catch (error) { setNotice((error as Error).message); }
  }
  return <section className="vibe-task-output" aria-label={brief ? "Unexecuted project brief text" : "Your agent’s output"}>
    <div className="vibe-output-preview" data-collapsed={long && !expanded || undefined} onFocusCapture={() => setExpanded(true)}><div ref={content} className="vibe-output-content"><AgentReply>{text}</AgentReply></div></div>
    {long && <VibeButton variant="quiet" aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>{expanded ? "Show less output" : "Show full output"}</VibeButton>}
    <div className="vibe-output-controls">
      <VibeButton variant="quiet" onClick={() => void navigator.clipboard.writeText(text).then(() => setNotice(brief ? "Copied brief" : "Copied full output")).catch(() => setNotice(brief ? "Select the brief and copy it." : "Select the output and copy it."))}>{brief ? "Copy brief" : "Copy output"}</VibeButton>
      <details className="vibe-output-download"><summary>{brief ? "Download brief" : "Download output"}</summary><div className="vibe-output-download-options">
        <VibeButton variant="quiet" onClick={() => download("markdown")}>Markdown (.md)</VibeButton>
        {hasJSON && <VibeButton variant="quiet" onClick={() => download("json")}>JSON (.json)</VibeButton>}
      </div></details>
    </div>
    <span role="status" className="text-xs vibe-muted">{notice}</span>
    {sessionID && materials.length > 0 && <TaskSources sessionID={sessionID} bindings={materials} />}
  </section>;
}
