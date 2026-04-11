"use client";

import { useAuth } from "@workos-inc/authkit-nextjs/components";

export function SignOutButton() {
  const { signOut } = useAuth();

  return (
    <button
      onClick={() => signOut()}
      style={{
        padding: "0.375rem 0.75rem",
        background: "rgba(255, 255, 255, 0.06)",
        border: "1px solid rgba(255, 255, 255, 0.1)",
        borderRadius: "6px",
        color: "rgba(255, 255, 255, 0.6)",
        fontSize: "0.8125rem",
        cursor: "pointer",
      }}
    >
      Sign out
    </button>
  );
}
