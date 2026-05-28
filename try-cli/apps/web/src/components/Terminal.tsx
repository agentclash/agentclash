import { useEffect, useRef, useCallback } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "xterm/css/xterm.css";

interface Props {
  sessionId: string | null;
  status: string;
  onReady?: () => void;
  onCommand?: (cmd: string) => void;
}

export default function TerminalView({ sessionId, status, onReady }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<FitAddon | null>(null);

  const connect = useCallback((id: string) => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = import.meta.env.DEV ? "localhost:3000" : window.location.host;
    const ws = new WebSocket(`${proto}//${host}/ws?sessionId=${id}`);
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    ws.onopen = () => {
      onReady?.();
      const term = termRef.current;
      const fit = fitRef.current;
      if (term && fit) {
        fit.fit();
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
      }
    };

    ws.onmessage = (ev) => {
      const term = termRef.current;
      if (!term) return;
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else {
        term.write(ev.data);
      }
    };

    ws.onclose = () => {
      termRef.current?.writeln("\r\n\x1b[33m[session disconnected]\x1b[0m");
    };
  }, [onReady]);

  useEffect(() => {
    if (!containerRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: '"IBM Plex Mono", monospace',
      fontSize: 14,
      lineHeight: 1.4,
      theme: {
        background: "#0d1117",
        foreground: "#c9d1d9",
        cursor: "#58a6ff",
        selectionBackground: "#264f78",
        black: "#484f58",
        red: "#ff7b72",
        green: "#3fb950",
        yellow: "#d29922",
        blue: "#58a6ff",
        magenta: "#bc8cff",
        cyan: "#39c5cf",
        white: "#b1bac4",
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

    return () => {
      window.removeEventListener("resize", onResize);
      wsRef.current?.close();
      term.dispose();
    };
  }, []);

  useEffect(() => {
    if (sessionId && status === "ready") {
      connect(sessionId);
    }
  }, [sessionId, status, connect]);

  return (
    <div className="terminal-wrap">
      {status === "starting" && (
        <div className="terminal-overlay">
          <div className="spinner" />
          <p>Starting sandbox…</p>
          <p className="muted">Installing tools — usually 15–30s first time</p>
        </div>
      )}
      <div ref={containerRef} className="terminal-container" />
    </div>
  );
}

export function runCommandInTerminal(cmd: string) {
  // Used by sidebar — dispatches via custom event
  window.dispatchEvent(new CustomEvent("try-cli-run", { detail: cmd }));
}
