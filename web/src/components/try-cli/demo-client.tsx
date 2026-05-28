"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ExternalLink, RotateCcw } from "lucide-react";
import { TryCliTerminal } from "@/components/try-cli/terminal";
import { getTryCliApiBase, tryCliPublicOrigin } from "@/lib/try-cli/config";
import type { DemoMeta, TrySession } from "@/lib/try-cli/types";

interface Props {
  slug: string;
  initialDemo?: DemoMeta | null;
}

export function TryCliDemoClient({ slug, initialDemo = null }: Props) {
  const apiBase = getTryCliApiBase();
  const [demo, setDemo] = useState<DemoMeta | null>(initialDemo);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState("loading");
  const [expiresAt, setExpiresAt] = useState(0);
  const [remaining, setRemaining] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const pollSession = useCallback(
    async (id: string) => {
      const poll = async () => {
        const res = await fetch(`${apiBase}/sessions/${id}`);
        const data = (await res.json()) as TrySession & { error?: string };
        setStatus(data.status);
        if (data.status === "starting") setTimeout(poll, 1500);
        if (data.status === "error") setError(data.error ?? "Sandbox failed");
      };
      poll();
    },
    [apiBase],
  );

  const createSession = useCallback(async () => {
    setStatus("starting");
    setError(null);
    const res = await fetch(`${apiBase}/sessions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug }),
    });
    const data = await res.json();
    if (!res.ok) {
      setError(data.error ?? "Failed to start session");
      setStatus("error");
      return;
    }
    setSessionId(data.id);
    setExpiresAt(data.expiresAt);
    pollSession(data.id);
  }, [slug, apiBase, pollSession]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const res = await fetch(`${apiBase}/demos/${slug}`);
      const d = (await res.json()) as DemoMeta & { error?: string };
      if (cancelled) return;
      if (d.error || !res.ok) {
        setError("Demo not found");
        setStatus("error");
        return;
      }
      setDemo(d);
      await createSession();
    })();
    return () => {
      cancelled = true;
    };
  }, [slug, apiBase, createSession]);

  useEffect(() => {
    if (!expiresAt) return;
    const tick = () => {
      const ms = expiresAt - Date.now();
      if (ms <= 0) {
        setRemaining("Expired");
        return;
      }
      const m = Math.floor(ms / 60000);
      const s = Math.floor((ms % 60000) / 1000);
      setRemaining(`${m}:${s.toString().padStart(2, "0")}`);
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [expiresAt]);

  const reset = async () => {
    if (!sessionId) return;
    const res = await fetch(`${apiBase}/sessions/${sessionId}?action=reset`, { method: "POST" });
    const data = await res.json();
    setSessionId(data.id);
    setExpiresAt(data.expiresAt);
    setStatus(data.status);
    if (data.status === "starting") pollSession(data.id);
  };

  const copyCmd = (cmd: string) => {
    void navigator.clipboard.writeText(cmd);
    setCopied(cmd);
    setTimeout(() => setCopied(null), 2000);
  };

  const publicOrigin = tryCliPublicOrigin().replace(/\/$/, "");
  const badgeMd = `[![Try on AgentClash](${publicOrigin}/api/try/badge/${slug}.svg)](${publicOrigin}/try/${slug})`;

  if (error && !demo) {
    return (
      <div className="mx-auto max-w-lg px-6 py-24 text-center">
        <h1 className="text-xl font-semibold">Demo not found</h1>
        <p className="mt-2 text-muted-foreground">{error}</p>
        <Link href="/try" className="mt-6 inline-block text-sm underline">
          ← All demos
        </Link>
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-4rem)] flex-col">
      <header className="flex shrink-0 items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-3">
          <Link href="/try" className="text-sm font-medium text-muted-foreground hover:text-foreground">
            AgentClash Try
          </Link>
          <span className="text-muted-foreground">/</span>
          <h1 className="text-sm font-semibold">{demo?.name ?? slug}</h1>
          {demo?.github && (
            <a href={demo.github} target="_blank" rel="noreferrer" className="text-xs text-muted-foreground hover:text-foreground">
              GitHub <ExternalLink className="inline size-3" />
            </a>
          )}
          {demo?.docs && (
            <a href={demo.docs} target="_blank" rel="noreferrer" className="text-xs text-muted-foreground hover:text-foreground">
              Docs <ExternalLink className="inline size-3" />
            </a>
          )}
        </div>
        <div className="flex items-center gap-3">
          <span className="font-mono text-xs text-muted-foreground">⏱ {remaining || "—"}</span>
          <button
            type="button"
            onClick={() => void reset()}
            className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs hover:bg-muted"
          >
            <RotateCcw className="size-3" />
            Reset
          </button>
        </div>
      </header>

      {error && (
        <div className="bg-destructive/10 px-4 py-2 text-sm text-destructive">{error}</div>
      )}

      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <aside className="w-full shrink-0 overflow-y-auto border-b border-border p-4 md:w-72 md:border-b-0 md:border-r">
          <h2 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            Suggested commands
          </h2>
          <p className="mt-1 text-xs text-muted-foreground">Click to copy, paste in terminal</p>
          <ul className="mt-3 space-y-2">
            {demo?.commands.map((c) => (
              <li key={c.run}>
                <button
                  type="button"
                  onClick={() => copyCmd(c.run)}
                  className="w-full rounded-md border border-border bg-muted/30 px-3 py-2 text-left text-sm hover:border-foreground/20"
                >
                  <span className="font-medium">{c.label}</span>
                  <code className="mt-1 block text-xs text-muted-foreground">{c.run}</code>
                  {copied === c.run && <span className="text-xs text-green-500">Copied</span>}
                </button>
              </li>
            ))}
          </ul>
          <div className="mt-6 border-t border-border pt-4">
            <h3 className="text-xs font-medium uppercase text-muted-foreground">README badge</h3>
            <code className="mt-2 block break-all rounded bg-muted/50 p-2 text-[10px]">{badgeMd}</code>
          </div>
        </aside>
        <main className="min-h-0 flex-1 p-4">
          <TryCliTerminal sessionId={sessionId} status={status} />
        </main>
      </div>
    </div>
  );
}
