"use client";

import { useEffect, useLayoutEffect, useRef, useState, type UIEvent } from "react";
import { buildTimeline } from "@/lib/vibe-build-timeline";
import type { Session } from "@/lib/vibe";

const positions = new Map<string, number>();

// The automatic check belongs below the answer, not in place of its reading
// position. Explicit later reruns and new messages still get their own anchor.
export function buildReadingAnchor(session: Session, pendingID?: string): string | undefined {
  if (pendingID) return `message:${pendingID}`;
  const last = buildTimeline(session).at(-1);
  if (last?.kind === "run" && last.operation.id === session.document.build?.check_id) {
    const reply = session.document.messages.findLast(message =>
      message.origin === "playground" && message.role === "assistant" &&
      message.operation_id === session.document.build?.trial_id &&
      message.artifact_id === last.artifact?.id,
    );
    if (reply) return `message:${reply.id}`;
  }
  return last?.id;
}

export function useConversationScroll({ sessionID, view, stamp, build, anchor, reduced }: {
  sessionID?: string;
  view: string;
  stamp: string;
  build: boolean;
  anchor?: string;
  reduced: boolean | null;
}) {
  const region = useRef<HTMLDivElement>(null);
  const end = useRef<HTMLDivElement>(null);
  const nearBottom = useRef(true);
  const [isNearBottom, setIsNearBottom] = useState(true);
  const key = `${sessionID || "entry"}:${view}`;
  const [read, setRead] = useState({ sessionID, stamp });
  const previous = useRef({ sessionID, stamp, anchor });
  const behavior = reduced ? "auto" : "smooth";

  function markRead() {
    setRead(old => old.sessionID === sessionID && old.stamp === stamp ? old : { sessionID, stamp });
  }
  function remember() {
    if (!region.current) return;
    positions.set(key, region.current.scrollTop);
    if (positions.size > 100) positions.delete(positions.keys().next().value!);
  }
  function follow() {
    nearBottom.current = true;
    setIsNearBottom(true);
  }
  function jumpToLatest() {
    follow();
    markRead();
    end.current?.scrollIntoView?.({ behavior, block: "nearest" });
  }
  function onScroll(event: UIEvent<HTMLDivElement>) {
    if (build) remember();
    const element = event.currentTarget;
    const wasNearBottom = nearBottom.current;
    nearBottom.current = element.scrollHeight - element.scrollTop - element.clientHeight < 120;
    setIsNearBottom(nearBottom.current);
    if (nearBottom.current || wasNearBottom) markRead();
  }

  useLayoutEffect(() => {
    const element = region.current;
    if (!element || !build) return;
    element.scrollTop = positions.get(key) || 0;
    nearBottom.current = element.scrollHeight - element.scrollTop - element.clientHeight < 120;
    setIsNearBottom(nearBottom.current);
  }, [key, build]);

  useEffect(() => {
    const before = previous.current;
    previous.current = { sessionID, stamp, anchor };
    if (before.sessionID !== sessionID || before.stamp === stamp || !nearBottom.current) return;
    if (!build) {
      end.current?.scrollIntoView?.({ behavior, block: "nearest" });
      return;
    }
    if (!anchor || before.anchor === anchor) return;
    const element = region.current;
    const target = Array.from(element?.querySelectorAll<HTMLElement>("[data-build-entry], [data-pending-message]") || [])
      .find(node => node.dataset.buildEntry === anchor || node.dataset.pendingMessage === anchor);
    if (element && target) {
      const top = target.getBoundingClientRect().top - element.getBoundingClientRect().top + element.scrollTop;
      element.scrollTo?.({ top: Math.min(top, element.scrollHeight - element.clientHeight), behavior });
    }
  }, [sessionID, stamp, anchor, build, behavior]);

  return { region, end, isNearBottom, onScroll, follow, markRead, remember, jumpToLatest,
    hasNewResponse: !isNearBottom && read.sessionID === sessionID && read.stamp !== stamp };
}
