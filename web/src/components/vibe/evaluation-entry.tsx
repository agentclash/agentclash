"use client";

import { motion, useReducedMotion } from "framer-motion";
import { ArrowRight, FlaskConical, Sparkles } from "lucide-react";

export function EvaluationEntry({ busy, onDoor }: { busy: boolean; onDoor?: (door: "build" | "test") => void }) {
  const reduced = useReducedMotion();
  return <motion.section className="vibe-welcome mb-7" initial={reduced ? false : { opacity: 0, y: 5 }}
    animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.18 }}>
    <h1 className="vibe-entry-title">What would you like AI to handle?</h1>
    <p className="mt-4 vibe-muted">Try an idea, or find out what your existing agent gets right and wrong.</p>
    <div className="vibe-entry-doors mt-7">
      <button type="button" disabled={busy} onClick={() => onDoor?.("build")}>
        <Sparkles className="vibe-door-icon" size={20} aria-hidden="true" />
        <span>Build an agent <ArrowRight size={18} aria-hidden="true" /></span>
        <small>I have something I want AI to handle.</small>
      </button>
      <button type="button" disabled={busy} onClick={() => onDoor?.("test")}>
        <FlaskConical className="vibe-door-icon" size={20} aria-hidden="true" />
        <span>Improve an existing agent <ArrowRight size={18} aria-hidden="true" /></span>
        <small>I already have an AI agent and want to find what to fix.</small>
      </button>
    </div>
  </motion.section>;
}
