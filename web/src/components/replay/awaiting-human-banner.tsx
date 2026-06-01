"use client";

import { useCallback, useEffect, useState } from "react";
import { createApiClient } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Panel } from "@/app/(workspace)/workspaces/[workspaceId]/runs/[runId]/agents/[runAgentId]/scorecard/components/panel";
import {
  getHumanTurnStatus,
  submitHumanTurn,
  type HumanTurnStatus,
} from "@/lib/api/multi-turn";
import { Loader2, UserRound } from "lucide-react";
import { toast } from "sonner";

interface AwaitingHumanBannerProps {
  getAccessToken: () => Promise<string | undefined>;
  workspaceId: string;
  runId: string;
  runAgentId: string;
  enabled: boolean;
}

export function AwaitingHumanBanner({
  getAccessToken,
  workspaceId,
  runId,
  runAgentId,
  enabled,
}: AwaitingHumanBannerProps) {
  const [status, setStatus] = useState<HumanTurnStatus | null>(null);
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const refresh = useCallback(async () => {
    if (!enabled) {
      setStatus(null);
      return;
    }
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const next = await getHumanTurnStatus(api, workspaceId, runId, runAgentId);
      setStatus(next);
    } catch {
      // Ignore polling errors; replay page stays usable.
    }
  }, [getAccessToken, enabled, workspaceId, runId, runAgentId]);

  useEffect(() => {
    void refresh();
    if (!enabled) return;
    const interval = setInterval(() => void refresh(), 3000);
    return () => clearInterval(interval);
  }, [enabled, refresh]);

  if (!enabled || !status?.awaiting_human) {
    return null;
  }

  async function handleSubmit() {
    const trimmed = message.trim();
    if (!trimmed) return;
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await submitHumanTurn(api, workspaceId, runId, runAgentId, trimmed);
      setMessage("");
      toast.success("Human turn submitted");
      await refresh();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to submit turn");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Panel className="mb-4 border-amber-500/40 bg-amber-500/5 p-4">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium">
        <UserRound className="size-4 text-amber-600" />
        Awaiting human input — turn {status.turn_index ?? "?"}
        {status.phase_id ? ` (${status.phase_id})` : ""}
      </div>
      {status.prompt_hint && (
        <p className="mb-3 text-sm text-muted-foreground">{status.prompt_hint}</p>
      )}
      <textarea
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        placeholder="Type the user message to continue the conversation…"
        rows={3}
        className="mb-3 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
      />
      <Button size="sm" onClick={() => void handleSubmit()} disabled={submitting}>
        {submitting && <Loader2 className="mr-1.5 size-4 animate-spin" />}
        Submit turn
      </Button>
    </Panel>
  );
}
