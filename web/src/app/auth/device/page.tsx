import { withAuth } from "@workos-inc/authkit-nextjs";
import { DeviceCodeForm } from "./device-code-form";

export default async function DeviceAuthPage() {
  const { user } = await withAuth({ ensureSignedIn: true });

  return (
    <div style={containerStyle}>
      <div style={cardStyle}>
        <div style={{ textAlign: "center", marginBottom: "1.5rem" }}>
          <div style={iconStyle}>
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
              <line x1="8" y1="21" x2="16" y2="21" />
              <line x1="12" y1="17" x2="12" y2="21" />
            </svg>
          </div>
          <h1 style={titleStyle}>Authorize Device</h1>
          <p style={descStyle}>
            Enter the code shown in your terminal to authorize the AgentClash CLI.
          </p>
        </div>

        <div style={userInfoStyle}>
          <div style={labelStyle}>Signed in as</div>
          <div style={valueStyle}>{user.email}</div>
        </div>

        <DeviceCodeForm />
      </div>
    </div>
  );
}

const containerStyle: React.CSSProperties = {
  minHeight: "100vh",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  padding: "2rem",
};

const cardStyle: React.CSSProperties = {
  width: "100%",
  maxWidth: "420px",
  background: "rgba(255, 255, 255, 0.03)",
  border: "1px solid rgba(255, 255, 255, 0.08)",
  borderRadius: "12px",
  padding: "2rem",
};

const titleStyle: React.CSSProperties = {
  fontFamily: "var(--font-display), serif",
  fontSize: "1.5rem",
  letterSpacing: "-0.02em",
  color: "rgba(255, 255, 255, 0.9)",
  margin: "0.75rem 0 0",
};

const descStyle: React.CSSProperties = {
  color: "rgba(255, 255, 255, 0.4)",
  fontSize: "0.875rem",
  marginTop: "0.5rem",
};

const iconStyle: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  width: "56px",
  height: "56px",
  borderRadius: "12px",
  background: "rgba(255, 255, 255, 0.05)",
  color: "rgba(255, 255, 255, 0.7)",
};

const userInfoStyle: React.CSSProperties = {
  background: "rgba(255, 255, 255, 0.03)",
  border: "1px solid rgba(255, 255, 255, 0.06)",
  borderRadius: "8px",
  padding: "1rem",
  marginBottom: "1.5rem",
};

const labelStyle: React.CSSProperties = {
  fontSize: "0.75rem",
  color: "rgba(255, 255, 255, 0.4)",
  marginBottom: "0.25rem",
  textTransform: "uppercase",
  letterSpacing: "0.05em",
};

const valueStyle: React.CSSProperties = {
  fontSize: "0.9375rem",
  color: "rgba(255, 255, 255, 0.9)",
};
