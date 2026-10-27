"use client";
import { useId, useState } from "react";
import { VibeButton } from "./vibe-button";
import { motion, useReducedMotion } from "framer-motion";
import { ClashMark } from "@/components/marketing/clash-mark";
import type { Session } from "@/lib/vibe";
import { SafeMarkdown } from "./safe-markdown";

export function Message({
  message,
  pending = false,
  animate = true,
}: {
  message: Session["document"]["messages"][number];
  pending?: boolean;
  animate?: boolean;
}) {
  const reduced = useReducedMotion();
  const [expanded, setExpanded] = useState(false);
  const id = useId();
  const long = message.content.length > 1600;
  return (
    <motion.div
      className={`vibe-message ${message.role === "user" ? "vibe-message-user" : ""}`}
      data-pending={pending || undefined}
      initial={!animate || reduced ? false : { opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: reduced ? 0 : 0.18 }}
    >
      {message.role !== "user" && (
        <p className="mb-2 flex items-center gap-2 text-xs vibe-muted">
          <ClashMark className="size-4" />
          Vibe Evals
        </p>
      )}
      <div id={id} className="vibe-reply-body" data-collapsed={long && !expanded || undefined} onFocusCapture={() => setExpanded(true)}><SafeMarkdown>{message.content}</SafeMarkdown></div>
      {long && <VibeButton aria-controls={id} aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>{expanded ? "Show less reply" : "Show full reply"}</VibeButton>}
    </motion.div>
  );
}
