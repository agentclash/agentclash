"use client";

import { useState } from "react";
import { approveCLILogin } from "./actions";

interface Props {
  port: number;
  state: string;
}

export function CLIApproveButton({ port, state }: Props) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleApprove() {
    setLoading(true);
    setError(null);
    try {
      const result = await approveCLILogin(port, state);
      if (result.error) {
        setError(result.error);
        setLoading(false);
        return;
      }
      window.location.href = result.redirectUrl!;
    } catch {
      setError("Failed to authorize. Please try again.");
      setLoading(false);
    }
  }

  function handleDeny() {
    window.location.href = `http://127.0.0.1:${port}/callback?error=access_denied&state=${encodeURIComponent(state)}`;
  }

  return (
    <div>
      {error && (
        <div style={errorStyle}>{error}</div>
      )}
      <button
        onClick={handleApprove}
        disabled={loading}
        style={{
          ...buttonStyle,
          background: loading ? "rgba(255, 255, 255, 0.5)" : "rgba(255, 255, 255, 0.9)",
          cursor: loading ? "not-allowed" : "pointer",
        }}
      >
        {loading ? "Authorizing..." : "Approve"}
      </button>
      <button
        onClick={handleDeny}
        disabled={loading}
        style={denyButtonStyle}
      >
        Deny
      </button>
    </div>
  );
}

const buttonStyle: React.CSSProperties = {
  display: "block",
  width: "100%",
  padding: "0.75rem 1rem",
  color: "#060606",
  borderRadius: "8px",
  fontWeight: 500,
  fontSize: "0.9375rem",
  textAlign: "center",
  border: "none",
  marginBottom: "0.75rem",
};

const denyButtonStyle: React.CSSProperties = {
  display: "block",
  width: "100%",
  padding: "0.75rem 1rem",
  background: "transparent",
  color: "rgba(255, 255, 255, 0.4)",
  borderRadius: "8px",
  fontWeight: 500,
  fontSize: "0.875rem",
  textAlign: "center",
  border: "1px solid rgba(255, 255, 255, 0.08)",
  cursor: "pointer",
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
