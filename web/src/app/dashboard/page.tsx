import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { SignOutButton } from "./sign-out-button";

export default async function DashboardPage() {
  const { user } = await withAuth();
  if (!user) redirect("/auth/login");

  return (
    <div style={{ minHeight: "100vh" }}>
      <header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 1.5rem",
          height: "56px",
          borderBottom: "1px solid rgba(255, 255, 255, 0.08)",
        }}
      >
        <a
          href="/dashboard"
          style={{
            fontFamily: "var(--font-display), serif",
            fontSize: "1.125rem",
            color: "rgba(255, 255, 255, 0.9)",
            textDecoration: "none",
          }}
        >
          AgentClash
        </a>
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          <span style={{ fontSize: "0.8125rem", color: "rgba(255, 255, 255, 0.5)" }}>
            {user.firstName || user.email}
          </span>
          <SignOutButton />
        </div>
      </header>

      <main style={{ padding: "2rem 1.5rem", maxWidth: "640px" }}>
        <h1
          style={{
            fontFamily: "var(--font-display), serif",
            fontSize: "1.5rem",
            color: "rgba(255, 255, 255, 0.9)",
            letterSpacing: "-0.02em",
            marginBottom: "1.5rem",
          }}
        >
          Dashboard
        </h1>

        <div
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "0.5rem",
            padding: "0.375rem 0.75rem",
            background: "rgba(34, 197, 94, 0.1)",
            border: "1px solid rgba(34, 197, 94, 0.2)",
            borderRadius: "6px",
            fontSize: "0.8125rem",
            color: "rgba(34, 197, 94, 0.9)",
            marginBottom: "2rem",
          }}
        >
          &#10003; Authenticated
        </div>

        <div
          style={{
            background: "rgba(255, 255, 255, 0.03)",
            border: "1px solid rgba(255, 255, 255, 0.08)",
            borderRadius: "12px",
            padding: "1.5rem",
            marginBottom: "1.5rem",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "1rem",
              marginBottom: "1.25rem",
              paddingBottom: "1rem",
              borderBottom: "1px solid rgba(255, 255, 255, 0.06)",
            }}
          >
            {user.profilePictureUrl ? (
              <img
                src={user.profilePictureUrl}
                alt=""
                style={{ width: 48, height: 48, borderRadius: "50%" }}
              />
            ) : (
              <div
                style={{
                  width: 48,
                  height: 48,
                  borderRadius: "50%",
                  background: "rgba(147, 130, 255, 0.15)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: "1.25rem",
                  color: "rgba(147, 130, 255, 0.9)",
                }}
              >
                {(user.firstName || user.email || "U").charAt(0).toUpperCase()}
              </div>
            )}
            <div>
              <div style={{ fontWeight: 500, color: "rgba(255, 255, 255, 0.9)" }}>
                {user.firstName} {user.lastName}
              </div>
              <div style={{ fontSize: "0.8125rem", color: "rgba(255, 255, 255, 0.4)" }}>
                {user.email}
              </div>
            </div>
          </div>

          <Row label="User ID" value={user.id} mono />
          <Row label="Email" value={user.email || "—"} />
          <Row label="First Name" value={user.firstName || "—"} />
          <Row label="Last Name" value={user.lastName || "—"} />
          <Row label="Email Verified" value={user.emailVerified ? "Yes" : "No"} last />
        </div>

        <details
          style={{
            background: "rgba(255, 255, 255, 0.03)",
            border: "1px solid rgba(255, 255, 255, 0.08)",
            borderRadius: "12px",
            padding: "1rem 1.25rem",
          }}
        >
          <summary
            style={{
              cursor: "pointer",
              fontSize: "0.8125rem",
              color: "rgba(255, 255, 255, 0.5)",
            }}
          >
            Raw user object
          </summary>
          <pre
            style={{
              marginTop: "1rem",
              padding: "1rem",
              background: "rgba(0, 0, 0, 0.3)",
              borderRadius: "8px",
              fontSize: "0.75rem",
              fontFamily: "var(--font-mono), monospace",
              color: "rgba(255, 255, 255, 0.6)",
              overflow: "auto",
              lineHeight: 1.6,
            }}
          >
            {JSON.stringify(user, null, 2)}
          </pre>
        </details>
      </main>
    </div>
  );
}

function Row({ label, value, mono, last }: { label: string; value: string; mono?: boolean; last?: boolean }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", padding: "0.5rem 0", borderBottom: last ? "none" : "1px solid rgba(255, 255, 255, 0.04)" }}>
      <span style={{ fontSize: "0.8125rem", color: "rgba(255, 255, 255, 0.4)" }}>{label}</span>
      <span style={{ fontSize: "0.8125rem", color: "rgba(255, 255, 255, 0.7)", fontFamily: mono ? "var(--font-mono), monospace" : "inherit", maxWidth: "320px", overflow: "hidden", textOverflow: "ellipsis" }}>{value}</span>
    </div>
  );
}
