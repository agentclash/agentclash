"use client";

import { useState } from "react";
import { authorizeDevice } from "./actions";

export function DeviceCodeForm() {
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("");

  function formatCode(raw: string): string {
    const clean = raw.toUpperCase().replace(/[^A-Z0-9]/g, "");
    if (clean.length > 4) {
      return clean.slice(0, 4) + "-" + clean.slice(4, 8);
    }
    return clean;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const cleaned = code.replace(/[^A-Z0-9]/g, "");
    if (cleaned.length !== 8) {
      setStatus("error");
      setMessage("Code must be 8 characters (e.g., RRGQ-BJVS)");
      return;
    }

    setLoading(true);
    setStatus("idle");

    const result = await authorizeDevice(code);
    setLoading(false);

    if (result.ok) {
      setStatus("success");
      setMessage("Device authorized! You can close this page and return to your terminal.");
    } else {
      setStatus("error");
      setMessage(result.error ?? "Failed to authorize device");
    }
  }

  if (status === "success") {
    return (
      <div style={successStyle}>
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginBottom: "0.5rem" }}>
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
          <polyline points="22 4 12 14.01 9 11.01" />
        </svg>
        <p style={{ margin: 0, fontWeight: 500 }}>{message}</p>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit}>
      <label style={inputLabelStyle}>Device Code</label>
      <input
        type="text"
        value={code}
        onChange={(e) => setCode(formatCode(e.target.value))}
        placeholder="XXXX-XXXX"
        maxLength={9}
        autoFocus
        style={inputStyle}
        autoComplete="off"
        spellCheck={false}
      />

      {status === "error" && (
        <div style={errorStyle}>{message}</div>
      )}

      <button
        type="submit"
        disabled={loading || code.replace(/-/g, "").length < 8}
        style={{
          ...buttonStyle,
          opacity: loading || code.replace(/-/g, "").length < 8 ? 0.5 : 1,
          cursor: loading ? "not-allowed" : "pointer",
        }}
      >
        {loading ? "Authorizing..." : "Authorize"}
      </button>
    </form>
  );
}

const inputLabelStyle: React.CSSProperties = {
  display: "block",
  fontSize: "0.75rem",
  color: "rgba(255, 255, 255, 0.4)",
  marginBottom: "0.5rem",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
};

const inputStyle: React.CSSProperties = {
  display: "block",
  width: "100%",
  padding: "0.875rem 1rem",
  background: "rgba(255, 255, 255, 0.05)",
  border: "1px solid rgba(255, 255, 255, 0.1)",
  borderRadius: "8px",
  color: "rgba(255, 255, 255, 0.9)",
  fontSize: "1.5rem",
  fontFamily: "monospace",
  textAlign: "center",
  letterSpacing: "0.15em",
  marginBottom: "1rem",
  outline: "none",
  boxSizing: "border-box",
};

const buttonStyle: React.CSSProperties = {
  display: "block",
  width: "100%",
  padding: "0.75rem 1rem",
  background: "rgba(255, 255, 255, 0.9)",
  color: "#060606",
  borderRadius: "8px",
  fontWeight: 500,
  fontSize: "0.9375rem",
  textAlign: "center",
  border: "none",
};

const errorStyle: React.CSSProperties = {
  background: "rgba(239, 68, 68, 0.1)",
  border: "1px solid rgba(239, 68, 68, 0.2)",
  borderRadius: "8px",
  padding: "0.75rem",
  color: "rgba(239, 68, 68, 0.9)",
  fontSize: "0.8125rem",
  marginBottom: "1rem",
};

const successStyle: React.CSSProperties = {
  textAlign: "center",
  color: "rgba(34, 197, 94, 0.9)",
  padding: "1.5rem",
};
