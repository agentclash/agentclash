"use client";
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
      <SafeMarkdown>{message.content}</SafeMarkdown>
    </motion.div>
  );
}
