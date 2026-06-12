"use client";

import type { ReactNode } from "react";
import type { TranscriptTurn } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Panel, PanelHeader } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/panel";
import { cn } from "@/lib/utils";
import {
  MessagesSquare,
  GraduationCap,
  Bot,
  User,
  ScrollText,
  AlertTriangle,
  Clock,
} from "lucide-react";

interface ConversationTranscriptProps {
  turns: TranscriptTurn[];
  /** Optional actions rendered in the panel header (e.g. the export button). */
  trailing?: ReactNode;
  /** Banner text shown above the conversation (e.g. a failed-run warning). */
  notice?: string;
}

/** Friendly label + icon for the simulated-user side of a turn. */
function studentIdentity(actor?: string): { label: string; icon: ReactNode } {
  switch (actor) {
    case "llm":
      return { label: "Student · LLM", icon: <GraduationCap className="size-3.5" /> };
    case "human":
      return { label: "Student · Human", icon: <User className="size-3.5" /> };
    case "scripted":
      return { label: "Student · Scripted", icon: <ScrollText className="size-3.5" /> };
    default:
      return { label: "Student", icon: <GraduationCap className="size-3.5" /> };
  }
}

export function ConversationTranscript({
  turns,
  trailing,
  notice,
}: ConversationTranscriptProps) {
  if (turns.length === 0) {
    return (
      <Panel className="overflow-hidden">
        <PanelHeader title="Conversation" icon={<MessagesSquare className="size-4" />} trailing={trailing} />
        {notice && (
          <div className="flex items-center gap-2 border-b border-amber-500/20 bg-amber-500/[0.05] px-4 py-2.5 text-xs text-amber-400/90">
            <AlertTriangle className="size-3.5 shrink-0" />
            <span>{notice}</span>
          </div>
        )}
        <EmptyState
          icon={<MessagesSquare className="size-10 text-muted-foreground" />}
          title="No conversation turns"
          description={
            notice
              ? "The run ended before any conversation turns were recorded."
              : "This run did not record a multi-turn conversation."
          }
        />
      </Panel>
    );
  }

  return (
    <Panel className="overflow-hidden">
      <PanelHeader
        title={`Conversation · ${turns.length} turn${turns.length === 1 ? "" : "s"}`}
        icon={<MessagesSquare className="size-4" />}
        trailing={trailing}
      />

      {notice && (
        <div className="flex items-center gap-2 border-b border-amber-500/20 bg-amber-500/[0.05] px-4 py-2.5 text-xs text-amber-400/90">
          <AlertTriangle className="size-3.5 shrink-0" />
          <span>{notice}</span>
        </div>
      )}

      <div className="divide-y divide-white/[0.05]">
        {turns.map((turn) => (
          <TurnBlock key={turn.turn_index} turn={turn} />
        ))}
      </div>
    </Panel>
  );
}

function TurnBlock({ turn }: { turn: TranscriptTurn }) {
  const student = studentIdentity(turn.actor);
  return (
    <div className="px-4 py-4 sm:px-5">
      {/* Turn header */}
      <div className="mb-3 flex items-center gap-2">
        <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.18em] text-white/35">
          Turn {turn.turn_index}
        </span>
        {turn.phase_id && (
          <Badge
            variant="outline"
            className="border-white/10 bg-white/[0.03] px-1.5 py-0 text-2xs font-normal text-white/55"
          >
            {turn.phase_id}
          </Badge>
        )}
        {turn.mismatch && (
          <Badge variant="destructive" className="px-1.5 py-0 text-2xs">
            mismatch
          </Badge>
        )}
        {turn.awaiting_human && (
          <Badge
            variant="outline"
            className="flex items-center gap-1 border-amber-500/30 bg-amber-500/[0.06] px-1.5 py-0 text-2xs font-normal text-amber-400/90"
          >
            <Clock className="size-3" /> awaiting human
          </Badge>
        )}
      </div>

      <div className="space-y-3">
        {turn.user_message && (
          <SpeakerRow
            icon={student.icon}
            label={student.label}
            accent="student"
            content={turn.user_message}
          />
        )}
        {turn.assistant_message && (
          <SpeakerRow
            icon={<Bot className="size-3.5" />}
            label="Agent"
            accent="agent"
            content={turn.assistant_message}
          />
        )}
        {turn.awaiting_human && turn.awaiting_human_hint && (
          <p className="pl-[1.6rem] text-xs italic text-amber-400/70">
            {turn.awaiting_human_hint}
          </p>
        )}
      </div>
    </div>
  );
}

function SpeakerRow({
  icon,
  label,
  accent,
  content,
}: {
  icon: ReactNode;
  label: string;
  accent: "student" | "agent";
  content: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div
        className={cn(
          "flex items-center gap-1.5 text-2xs uppercase tracking-[0.16em]",
          accent === "student" ? "text-cyan-300/70" : "text-violet-300/70",
        )}
      >
        <span className={accent === "student" ? "text-cyan-300/80" : "text-violet-300/80"}>
          {icon}
        </span>
        {label}
      </div>
      <div
        className={cn(
          "rounded-md border px-3.5 py-2.5 text-sm leading-relaxed whitespace-pre-wrap break-words",
          accent === "student"
            ? "border-cyan-400/15 bg-cyan-400/[0.04] text-white/85"
            : "border-violet-400/15 bg-violet-400/[0.04] text-white/85",
        )}
      >
        {content}
      </div>
    </div>
  );
}
