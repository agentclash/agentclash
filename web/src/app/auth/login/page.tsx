import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { SignInButton } from "./sign-in-button";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ returnTo?: string }>;
}) {
  const { returnTo: rawReturnTo } = await searchParams;
  const returnTo = sanitizeReturnTo(rawReturnTo);
  const { user } = await withAuth();
  if (user) redirect(returnTo);

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "2rem",
      }}
    >
      <div style={{ width: "100%", maxWidth: "420px" }}>
        <div style={{ textAlign: "center", marginBottom: "2.5rem" }}>
          <h1
            style={{
              fontFamily: "var(--font-display), serif",
              fontSize: "2rem",
              letterSpacing: "-0.02em",
              color: "rgba(255, 255, 255, 0.9)",
              margin: 0,
            }}
          >
            AgentClash
          </h1>
          <p
            style={{
              color: "rgba(255, 255, 255, 0.4)",
              fontSize: "0.875rem",
              marginTop: "0.5rem",
            }}
          >
            Sign in to your account
          </p>
        </div>

        <div
          style={{
            background: "rgba(255, 255, 255, 0.03)",
            border: "1px solid rgba(255, 255, 255, 0.08)",
            borderRadius: "12px",
            padding: "2rem",
          }}
        >
          <SignInButton returnTo={returnTo} />
        </div>
      </div>
    </div>
  );
}
