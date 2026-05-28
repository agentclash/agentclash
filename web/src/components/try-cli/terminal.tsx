"use client";

import { useEffect, useRef, useCallback } from "react";
import { getTryCliWsBase } from "@/lib/try-cli/config";

interface Props {
  sessionId: string | null;
  status: string;
  onReady?: () => void;
}

export function TryCliTerminal({ sessionId, status, onReady }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import("xterm").Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<import("@xterm/addon-fit").FitAddon | null>(null);

  const connect = useCallback(
    (id: string) => {
      wsRef.current?.close();
      const wsBase = getTryCliWsBase();
      const ws = new WebSocket(`${wsBase}/ws?sessionId=${id}`);
      ws.binaryType = "arraybuffer";
      wsRef.current = ws;

      ws.onopen = () => {
        onReady?.();
        const term = termRef.current;
        const fit = fitRef.current;
        if (term && fit) {
          fit.fit();
        }
      };

      ws.onmessage = (ev) => {
        const term = termRef.current;
        if (!term) return;
        if (ev.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(ev.data));
        } else {
          term.write(String(ev.data));
        }
      };

      ws.onclose = () => {
        termRef.current?.writeln("\r\n\x1b[33m[session disconnected]\x1b[0m");
      };
    },
    [onReady],
  );

  useEffect(() => {
    let disposed = false;

    async function init() {
      const [{ Terminal }, { FitAddon }, { WebLinksAddon }] = await Promise.all([
        import("xterm"),
        import("@xterm/addon-fit"),
        import("@xterm/addon-web-links"),
      ]);
      await import("xterm/css/xterm.css");

      if (disposed || !containerRef.current) return;

      const term = new Terminal({
        cursorBlink: true,
        fontFamily: "var(--font-mono, ui-monospace, monospace)",
        fontSize: 14,
        theme: {
          background: "#0a0a0a",
          foreground: "#e5e5e5",
          cursor: "#fafafa",
        },
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.loadAddon(new WebLinksAddon());
      term.open(containerRef.current);
      fit.fit();

      term.onData((data) => {
        if (wsRef.current?.readyState === WebSocket.OPEN) {
          wsRef.current.send(data);
        }
      });

      termRef.current = term;
      fitRef.current = fit;

      const onResize = () => fit.fit();
      window.addEventListener("resize", onResize);
      return () => window.removeEventListener("resize", onResize);
    }

    const cleanupResize = init();

    return () => {
      disposed = true;
      wsRef.current?.close();
      termRef.current?.dispose();
      void cleanupResize;
    };
  }, []);

  useEffect(() => {
    if (sessionId && (status === "ready" || status === "starting")) {
      connect(sessionId);
    }
  }, [sessionId, status, connect]);

  return (
    <div className="relative h-full min-h-[320px] w-full rounded-lg border border-border bg-[#0a0a0a]">
      {status === "starting" && (
        <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-black/80 text-sm text-muted-foreground">
          <div className="size-8 animate-spin rounded-full border-2 border-muted border-t-foreground" />
          <p>Starting sandbox…</p>
          <p className="text-xs">First launch may take 15–30s while tools install</p>
        </div>
      )}
      <div ref={containerRef} className="h-full w-full p-2" />
    </div>
  );
}
