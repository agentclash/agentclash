"use client";

import { signInAction } from "./actions";

export function SignInButton() {
  return (
    <form action={signInAction}>
      <button
        type="submit"
        style={{
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
          cursor: "pointer",
        }}
      >
        Sign in with WorkOS
      </button>
    </form>
  );
}
