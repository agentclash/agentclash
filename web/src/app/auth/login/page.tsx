import { getSignInUrl, withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";

export default async function LoginPage() {
  const { user } = await withAuth();
  if (user) redirect("/dashboard");

  const signInUrl = await getSignInUrl();

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
          <a
            href={signInUrl}
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
              textDecoration: "none",
            }}
          >
            Sign in with WorkOS
          </a>
        </div>
      </div>
    </div>
  );
}
