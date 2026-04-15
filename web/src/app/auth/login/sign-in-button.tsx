"use client";

import { signInAction } from "./actions";

interface SignInButtonProps {
  label?: string;
  returnTo?: string;
}

export function SignInButton({
  label = "Sign in with WorkOS",
  returnTo = "/dashboard",
}: SignInButtonProps) {
  return (
    <form action={signInAction}>
      <input type="hidden" name="returnTo" value={returnTo} />
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
        {label}
      </button>
    </form>
  );
}
