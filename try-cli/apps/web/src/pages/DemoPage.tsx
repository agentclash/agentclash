import { useEffect, useState, useCallback } from "react";
import TerminalView from "../components/Terminal";
import type { DemoMeta } from "../lib/types";

interface Props {
  slug: string;
}

export default function DemoPage({ slug }: Props) {
  const [demo, setDemo] = useState<DemoMeta | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState<string>("loading");
  const [expiresAt, setExpiresAt] = useState<number>(0);
  const [remaining, setRemaining] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const apiBase = import.meta.env.DEV ? "http://localhost:3000" : "";

  const createSession = useCallback(async () => {
    setStatus("starting");
    setError(null);
    const res = await fetch(`${apiBase}/api/sessions`, {
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
  }, [slug, apiBase]);

  const pollSession = useCallback(async (id: string) => {
    const poll = async () => {
      const res = await fetch(`${apiBase}/api/sessions/${id}`);
      const data = await res.json();
      setStatus(data.status);
      if (data.status === "starting") {
        setTimeout(poll, 1500);
      } else if (data.status === "error") {
        setError(data.error ?? "Sandbox failed");
      }
    };
    poll();
  }, [apiBase]);

  useEffect(() => {
    fetch(`${apiBase}/api/demos/${slug}`)
      .then((r) => r.json())
      .then((d) => {
        if (d.error) {
          setError("Demo not found");
          setStatus("error");
        } else {
          setDemo(d);
          createSession();
        }
      });
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
    const res = await fetch(`${apiBase}/api/sessions/${sessionId}?action=reset`, { method: "POST" });
    const data = await res.json();
    setSessionId(data.id);
    setExpiresAt(data.expiresAt);
    setStatus(data.status);
    if (data.status === "starting") pollSession(data.id);
  };

  const copyCmd = (cmd: string) => {
    navigator.clipboard.writeText(cmd);
    setCopied(cmd);
    setTimeout(() => setCopied(null), 2000);
  };

  const sendCmd = (cmd: string) => {
    copyCmd(cmd);
    // Terminal receives via paste simulation — user runs manually or we document click-to-copy
  };

  if (error && !demo) {
    return (
      <div className="page error-page">
        <h1>Demo not found</h1>
        <p>{error}</p>
        <a href="/">← Back to demos</a>
      </div>
    );
  }

  return (
    <div className="demo-page">
      <header className="demo-header">
        <div className="demo-header-left">
          <a href="/" className="logo">try-cli</a>
          <span className="divider">/</span>
          <h1>{demo?.name ?? slug}</h1>
          {demo?.github && (
            <a href={demo.github} target="_blank" rel="noreferrer" className="link-btn">
              GitHub ↗
            </a>
          )}
          {demo?.docs && (
            <a href={demo.docs} target="_blank" rel="noreferrer" className="link-btn">
              Docs ↗
            </a>
          )}
        </div>
        <div className="demo-header-right">
          <span className="timer" title="Session expires">⏱ {remaining || "—"}</span>
          <button type="button" className="btn secondary" onClick={reset}>Reset sandbox</button>
        </div>
      </header>

      {error && <div className="banner error">{error}</div>}

      <div className="demo-body">
        <aside className="sidebar">
          <h2>Suggested commands</h2>
          <p className="sidebar-hint">Click to copy, then paste in terminal</p>
          <ul className="cmd-list">
            {demo?.commands.map((c) => (
              <li key={c.run}>
                <button type="button" className="cmd-btn" onClick={() => sendCmd(c.run)}>
                  <span className="cmd-label">{c.label}</span>
                  <code className="cmd-run">{c.run}</code>
                  {copied === c.run && <span className="copied">Copied!</span>}
                </button>
              </li>
            ))}
          </ul>

          <div className="badge-box">
            <h3>Add to your README</h3>
            <code className="badge-code">{`[![Try CLI](${apiBase || "https://try-cli.dev"}/badge/${slug}.svg)](${apiBase || "https://try-cli.dev"}/${slug})`}</code>
          </div>
        </aside>

        <main className="terminal-main">
          <TerminalView sessionId={sessionId} status={status} />
        </main>
      </div>
    </div>
  );
}
