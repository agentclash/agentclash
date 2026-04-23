/**
 * LANDING 2 — Centered editorial layout, Resend-inspired
 * Serif headlines, centered content, warm palette, product preview table
 */
"use client";

import { Instrument_Serif, DM_Sans, JetBrains_Mono } from "next/font/google";
import ThemeToggle from "../components/ThemeToggle";
import ClashIcon3D from "../components/ClashIcon3D";

const instrumentSerif = Instrument_Serif({ weight: "400", subsets: ["latin"], variable: "--font-display" });
const dmSans = DM_Sans({ subsets: ["latin"], variable: "--font-body" });
const jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", weight: ["400", "500", "600"] });

export default function Landing2() {
  return (
    <div className={`${instrumentSerif.variable} ${dmSans.variable} ${jetbrainsMono.variable}`}>
      <style>{`
        :root[data-theme="dark"] .lp {
          --bg: #14120F;
          --bg-alt: #1A1815;
          --border: rgba(237,233,225,0.06);
          --text-1: #EDE9E1;
          --text-2: rgba(237,233,225,0.55);
          --text-3: rgba(237,233,225,0.28);
          --text-4: rgba(237,233,225,0.12);
          --accent: #D97757;
          --surface: rgba(237,233,225,0.03);
        }
        :root[data-theme="light"] .lp {
          --bg: #FAF8F5;
          --bg-alt: #F0EDE8;
          --border: rgba(20,18,15,0.08);
          --text-1: #1A1815;
          --text-2: rgba(26,24,21,0.55);
          --text-3: rgba(26,24,21,0.28);
          --text-4: rgba(26,24,21,0.1);
          --accent: #C4593C;
          --surface: rgba(20,18,15,0.025);
        }
        .lp {
          background: var(--bg);
          color: var(--text-1);
          font-family: var(--font-body), system-ui;
          transition: background 0.4s, color 0.4s;
        }
        .lp a { color: inherit; text-decoration: none; }
      `}</style>

      <div className="lp min-h-screen">

        {/* Nav */}
        <nav style={{
          maxWidth: "960px",
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "20px 32px",
        }}>
          <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
            <ClashIcon3D size={36} />
            <span style={{ fontFamily: "var(--font-display)", fontSize: "20px" }}>
              agent<span style={{ color: "var(--accent)" }}>clash</span>
            </span>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: "32px", fontSize: "14px", color: "var(--text-3)" }}>
            <a href="#">Docs</a>
            <a href="#">Pricing</a>
            <a href="#" style={{
              padding: "7px 20px",
              borderRadius: "6px",
              background: "var(--text-1)",
              color: "var(--bg)",
              fontWeight: 500,
              fontSize: "13px",
            }}>Sign in</a>
          </div>
        </nav>

        {/* Hero — centered */}
        <section style={{ maxWidth: "640px", margin: "0 auto", padding: "64px 32px 72px", textAlign: "center" }}>
          {/* 3D Icon */}
          <div style={{
            display: "flex",
            justifyContent: "center",
            marginBottom: "32px",
            filter: "drop-shadow(0 20px 40px rgba(217, 119, 87, 0.15))",
          }}>
            <ClashIcon3D size={140} />
          </div>

          <div style={{ display: "inline-flex", alignItems: "center", gap: "10px", marginBottom: "36px" }}>
            <span style={{ display: "block", width: "28px", height: "1.5px", background: "var(--accent)" }} />
            <span style={{ fontSize: "12px", fontWeight: 500, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--accent)" }}>
              Agent evaluation platform
            </span>
            <span style={{ display: "block", width: "28px", height: "1.5px", background: "var(--accent)" }} />
          </div>

          <h1 style={{
            fontFamily: "var(--font-display)",
            fontSize: "clamp(2.6rem, 6vw, 3.8rem)",
            lineHeight: 1.1,
            marginBottom: "28px",
          }}>
            Know which agent is{" "}
            <em style={{ color: "var(--accent)" }}>actually</em> better.
          </h1>

          <p style={{ fontSize: "17px", lineHeight: 1.8, color: "var(--text-2)", maxWidth: "480px", margin: "0 auto 40px" }}>
            Run agents on frozen benchmarks. Score them deterministically.
            Compare baseline against candidate. Ship with a verdict, not a feeling.
          </p>

          <div style={{ display: "flex", justifyContent: "center", alignItems: "center", gap: "16px" }}>
            <button style={{
              padding: "12px 28px",
              borderRadius: "6px",
              background: "var(--text-1)",
              color: "var(--bg)",
              fontSize: "14px",
              fontWeight: 500,
              border: "none",
              cursor: "pointer",
            }}>
              Start for free
            </button>
            <span style={{ fontSize: "13px", color: "var(--text-3)" }}>No credit card</span>
          </div>
        </section>

        {/* Divider */}
        <div style={{ maxWidth: "960px", margin: "0 auto", padding: "0 32px" }}>
          <div style={{ height: "1px", background: "var(--border)" }} />
        </div>

        {/* What it does — centered prose */}
        <section style={{ maxWidth: "600px", margin: "0 auto", padding: "64px 32px", textAlign: "center" }}>
          <p style={{ fontSize: "11px", fontWeight: 600, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--text-4)", marginBottom: "24px" }}>
            What AgentClash does
          </p>
          <p style={{ fontSize: "18px", lineHeight: 1.9, color: "var(--text-2)" }}>
            You changed the prompt, the model, or the tooling.{" "}
            <span style={{ color: "var(--text-1)" }}>
              AgentClash tells you if that change was a regression — with evidence.
            </span>{" "}
            It runs your agents on the same challenge, in isolated sandboxes,
            with the same validators, and produces a structured comparison:{" "}
            <span style={{ color: "var(--text-1)" }}>
              correctness delta, reliability delta, latency delta.
            </span>{" "}
            Your CI pipeline gets a verdict.{" "}
            <span style={{ color: "var(--accent)" }}>Pass, warn, or fail.</span>
          </p>
        </section>

        {/* Divider */}
        <div style={{ maxWidth: "960px", margin: "0 auto", padding: "0 32px" }}>
          <div style={{ height: "1px", background: "var(--border)" }} />
        </div>

        {/* How it works — centered columns */}
        <section style={{ background: "var(--bg-alt)", borderTop: "1px solid var(--border)", borderBottom: "1px solid var(--border)", padding: "72px 0" }}>
          <div style={{ maxWidth: "800px", margin: "0 auto", padding: "0 32px" }}>
            <p style={{ fontSize: "11px", fontWeight: 600, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--text-4)", marginBottom: "40px", textAlign: "center" }}>
              How it works
            </p>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: "48px" }}>
              {[
                {
                  title: "Define your benchmark",
                  desc: "A challenge pack with tasks, validators, and expected outputs. Versioned and immutable — the same version produces the same scores forever.",
                },
                {
                  title: "Race your agents",
                  desc: "Run 2–8 models in parallel. Each gets its own sandbox with real tool execution. Every decision captured as a canonical event.",
                },
                {
                  title: "Read the verdict",
                  desc: "Correctness: 1.0 vs 0.8. Reliability: passed. Tokens: 847 vs 1,178. Deltas per dimension. Release gate: pass or fail.",
                },
              ].map((step, i) => (
                <div key={i} style={{ textAlign: "center" }}>
                  <span style={{
                    fontFamily: "var(--font-display)",
                    fontSize: "42px",
                    lineHeight: 1,
                    color: "var(--text-4)",
                    display: "block",
                    marginBottom: "16px",
                  }}>
                    {i + 1}
                  </span>
                  <h3 style={{ fontSize: "16px", fontWeight: 600, marginBottom: "8px" }}>
                    {step.title}
                  </h3>
                  <p style={{ fontSize: "14px", lineHeight: 1.7, color: "var(--text-2)" }}>
                    {step.desc}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Product preview — centered comparison table */}
        <section style={{ maxWidth: "640px", margin: "0 auto", padding: "64px 32px" }}>
          <p style={{ fontSize: "11px", fontWeight: 600, letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--text-4)", marginBottom: "24px", textAlign: "center" }}>
            What you see after a race
          </p>

          <div style={{
            borderRadius: "10px",
            border: "1px solid var(--border)",
            overflow: "hidden",
            fontFamily: "var(--font-mono), monospace",
            fontSize: "13px",
          }}>
            {/* Header */}
            <div style={{
              padding: "10px 16px",
              borderBottom: "1px solid var(--border)",
              background: "var(--surface)",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}>
              <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <div style={{ display: "flex", gap: "5px" }}>
                  <span style={{ width: "8px", height: "8px", borderRadius: "50%", background: "var(--text-4)" }} />
                  <span style={{ width: "8px", height: "8px", borderRadius: "50%", background: "var(--text-4)" }} />
                  <span style={{ width: "8px", height: "8px", borderRadius: "50%", background: "var(--text-4)" }} />
                </div>
                <span style={{ fontSize: "11px", color: "var(--text-3)", marginLeft: "8px" }}>run comparison</span>
              </div>
              <span style={{ fontSize: "11px", color: "var(--text-4)" }}>fix-auth-bug · v1</span>
            </div>

            {/* Column headers */}
            <div style={{
              display: "grid",
              gridTemplateColumns: "120px 80px 80px 80px",
              padding: "6px 16px",
              borderBottom: "1px solid var(--border)",
              fontSize: "10px",
              fontWeight: 500,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
              color: "var(--text-4)",
            }}>
              <span></span>
              <span style={{ textAlign: "right" }}>base</span>
              <span style={{ textAlign: "right" }}>cand</span>
              <span style={{ textAlign: "right" }}>delta</span>
            </div>

            {[
              { dim: "correctness", base: "1.0000", cand: "1.0000", delta: " 0.00", color: "var(--text-3)" },
              { dim: "reliability", base: "1.0000", cand: "0.8000", delta: "−0.20", color: "var(--accent)" },
              { dim: "tokens", base: "   847", cand: " 1,178", delta: " +331", color: "var(--text-2)" },
              { dim: "latency", base: "  2.1s", cand: "  3.4s", delta: "+1.3s", color: "var(--text-2)" },
            ].map((row, i) => (
              <div key={i} style={{
                display: "grid",
                gridTemplateColumns: "120px 80px 80px 80px",
                padding: "7px 16px",
                borderBottom: i < 3 ? "1px solid var(--border)" : "none",
              }}>
                <span style={{ color: "var(--text-2)" }}>{row.dim}</span>
                <span style={{ textAlign: "right", color: "var(--text-3)" }}>{row.base}</span>
                <span style={{ textAlign: "right", color: "var(--text-3)" }}>{row.cand}</span>
                <span style={{ textAlign: "right", fontWeight: 600, color: row.color }}>{row.delta}</span>
              </div>
            ))}

            {/* Verdict */}
            <div style={{
              padding: "10px 16px",
              borderTop: "1px solid var(--border)",
              background: "var(--surface)",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}>
              <span style={{ fontSize: "11px", color: "var(--text-3)" }}>verdict</span>
              <span style={{
                fontSize: "11px",
                fontWeight: 600,
                color: "var(--accent)",
              }}>
                WARN — reliability regression
              </span>
            </div>
          </div>

          <p style={{ fontSize: "13px", color: "var(--text-3)", marginTop: "16px", lineHeight: 1.6, textAlign: "center", fontFamily: "var(--font-body), system-ui" }}>
            The candidate tied on correctness but regressed on reliability.
            Every number is backed by replay evidence.
          </p>
        </section>

        {/* Divider */}
        <div style={{ maxWidth: "960px", margin: "0 auto", padding: "0 32px" }}>
          <div style={{ height: "1px", background: "var(--border)" }} />
        </div>

        {/* The pitch */}
        <section style={{ maxWidth: "540px", margin: "0 auto", padding: "64px 32px", textAlign: "center" }}>
          <h2 style={{
            fontFamily: "var(--font-display)",
            fontSize: "clamp(1.6rem, 3.5vw, 2rem)",
            lineHeight: 1.4,
            marginBottom: "20px",
          }}>
            Tracing tools show you what happened.
            <br />
            <span style={{ color: "var(--text-3)" }}>AgentClash tells you what to do about it.</span>
          </h2>
          <p style={{ fontSize: "16px", lineHeight: 1.8, color: "var(--text-2)", marginBottom: "24px" }}>
            This is not an observability platform. It&apos;s regression testing for AI agents.
            You define what correct means. We tell you if your new version is better or worse — with evidence.
          </p>
          <a href="#" style={{ fontSize: "13px", fontWeight: 600, borderBottom: "1.5px solid var(--text-1)", paddingBottom: "2px" }}>
            Read the documentation →
          </a>
        </section>

        {/* CTA */}
        <section style={{
          background: "var(--bg-alt)",
          borderTop: "1px solid var(--border)",
          padding: "72px 32px",
          textAlign: "center",
        }}>
          <h2 style={{
            fontFamily: "var(--font-display)",
            fontSize: "clamp(1.8rem, 4vw, 2.4rem)",
            lineHeight: 1.2,
            marginBottom: "12px",
          }}>
            Ship agents with <em style={{ color: "var(--accent)" }}>proof</em>.
          </h2>
          <p style={{ color: "var(--text-2)", marginBottom: "28px", fontSize: "16px" }}>
            Free to start. No credit card.
          </p>
          <button style={{
            padding: "12px 28px",
            borderRadius: "6px",
            background: "var(--text-1)",
            color: "var(--bg)",
            fontSize: "14px",
            fontWeight: 500,
            border: "none",
            cursor: "pointer",
          }}>
            Create your workspace →
          </button>
        </section>

      </div>
      <ThemeToggle />
    </div>
  );
}
