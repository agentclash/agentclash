"use client";

import { useEffect, useRef, useCallback } from "react";
import "@xterm/xterm/css/xterm.css";
import { getTryCliWsBase } from "@/lib/try-cli/config";

interface Props {
  sessionId: string | null;
  status: string;
  onReady?: () => void;
}

export function TryCliTerminal({ sessionId, status, onReady }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<import("@xterm/xterm").Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<import("@xterm/addon-fit").FitAddon | null>(null);
  // The session id we've already opened a socket for. The server holds a
  // socket opened during "starting" and attaches the PTY once the sandbox is
  // ready, so we must connect exactly once per id — reconnecting on the
  // starting→ready transition is what produced the "[session disconnected]"
  // flash and a duplicated welcome banner.
  const connectedIdRef = useRef<string | null>(null);

  const connect = useCallback(
    (id: string) => {
      // Detach the previous socket's handler so an intentional swap (reset)
      // doesn't print the disconnect notice.
      const prev = wsRef.current;
      if (prev) {
        prev.onclose = null;
        prev.close();
      }
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
    let removeResizeListener: (() => void) | undefined;

    async function init() {
      const [{ Terminal }, { FitAddon }, { WebLinksAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
        import("@xterm/addon-web-links"),
      ]);

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
      removeResizeListener = () => window.removeEventListener("resize", onResize);

      // The effect may have been torn down while the dynamic imports were in
      // flight; if so, unregister immediately rather than leaking the listener.
      if (disposed) removeResizeListener();
    }

    void init();

    return () => {
      disposed = true;
      removeResizeListener?.();
      const ws = wsRef.current;
      if (ws) {
        ws.onclose = null;
        ws.close();
      }
      termRef.current?.dispose();
      // Allow a remount to reconnect to the same session id (refs survive an
      // effect cleanup, e.g. React strict-mode mount→cleanup→remount).
      connectedIdRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!sessionId) return;
    if (status !== "ready" && status !== "starting") return;
    // Connect once per session id; the server attaches the PTY when the
    // sandbox becomes ready, so we don't reconnect on the status change.
    if (connectedIdRef.current === sessionId) return;
    connectedIdRef.current = sessionId;
    connect(sessionId);
  }, [sessionId, status, connect]);

  return (
    <div className="relative flex h-full min-h-[320px] w-full flex-col overflow-hidden rounded-xl border border-white/[0.1] bg-[#0a0a0a] shadow-[0_0_0_1px_rgba(0,0,0,0.4),0_24px_60px_-24px_rgba(0,0,0,0.8)]">
      <div className="flex shrink-0 items-center gap-2 border-b border-white/[0.06] bg-white/[0.02] px-4 py-2.5">
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="size-2.5 rounded-full bg-white/15" />
        <span className="ml-2 font-[family-name:var(--font-mono)] text-2xs text-white/30">
          e2b · disposable sandbox
        </span>
      </div>
      <div className="relative min-h-0 flex-1">
        {status === "starting" && (
          <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-[#0a0a0a]/85 text-sm text-white/60 backdrop-blur-sm">
            <div className="size-7 animate-spin rounded-full border-2 border-white/15 border-t-white/70" />
            <p className="font-[family-name:var(--font-mono)] text-xs tracking-wide">
              booting sandbox…
            </p>
          </div>
        )}
        <div ref={containerRef} className="h-full w-full p-3" />
      </div>
    </div>
  );
}
