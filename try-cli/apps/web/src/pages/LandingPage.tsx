import { useEffect, useState } from "react";
import type { DemoMeta } from "../lib/types";

interface Props {
  onNavigate: (slug: string) => void;
}

export default function LandingPage({ onNavigate }: Props) {
  const [demos, setDemos] = useState<DemoMeta[]>([]);
  const apiBase = import.meta.env.DEV ? "http://localhost:3000" : "";

  useEffect(() => {
    fetch(`${apiBase}/api/demos`).then((r) => r.json()).then(setDemos);
  }, [apiBase]);

  return (
    <div className="landing">
      <header className="hero">
        <div className="hero-badge">Interactive CLI demos</div>
        <h1>Add a live terminal demo to your README in 60 seconds</h1>
        <p className="hero-sub">
          Let users try your CLI before they install it. One badge, zero setup for visitors,
          disposable Linux sandbox per session.
        </p>
        <div className="hero-cta">
          <code>npx try-cli init && npx try-cli publish</code>
        </div>
      </header>

      <section className="demos-grid-section">
        <h2>Try it now</h2>
        <div className="demos-grid">
          {demos.map((d) => (
            <button key={d.slug} type="button" className="demo-card" onClick={() => onNavigate(d.slug)}>
              <span className="demo-card-name">{d.name}</span>
              <span className="demo-card-cta">Open terminal →</span>
            </button>
          ))}
        </div>
      </section>

      <section className="how-it-works">
        <h2>How it works</h2>
        <ol>
          <li>Add <code>.trycli.yml</code> to your repo with install steps and suggested commands</li>
          <li>Run <code>npx try-cli publish</code> to get a README badge</li>
          <li>Users click → real Linux terminal opens → your CLI is pre-installed</li>
        </ol>
      </section>

      <footer className="landing-footer">
        <p>Built with E2B sandboxes + xterm.js · Sessions expire in 10 minutes</p>
      </footer>
    </div>
  );
}
