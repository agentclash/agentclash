import { withAuth } from "@workos-inc/authkit-nextjs";
import { CLIApproveButton } from "./cli-approve-button";

interface SearchParams {
  port?: string;
  state?: string;
}

export default async function CLILoginPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const params = await searchParams;
  const { user } = await withAuth({ ensureSignedIn: true });

  const port = params.port;
  const state = params.state;

  if (!port || !state) {
    return (
      <div style={containerStyle}>
        <div style={cardStyle}>
          <h1 style={titleStyle}>Invalid Request</h1>
          <p style={descStyle}>
            This page is used by the AgentClash CLI. Run{" "}
            <code style={codeStyle}>agentclash auth login</code> to authenticate.
          </p>
        </div>
      </div>
    );
  }

  const portNum = parseInt(port, 10);
  if (isNaN(portNum) || portNum < 1024 || portNum > 65535) {
    return (
      <div style={containerStyle}>
        <div style={cardStyle}>
          <h1 style={titleStyle}>Invalid Port</h1>
          <p style={descStyle}>The callback port is not valid.</p>
        </div>
      </div>
    );
  }

  return (
    <div style={containerStyle}>
      <div style={cardStyle}>
        <div style={{ textAlign: "center", marginBottom: "1.5rem" }}>
          <div style={iconStyle}>
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="4 17 10 11 4 5" />
              <line x1="12" y1="19" x2="20" y2="19" />
            </svg>
          </div>
          <h1 style={titleStyle}>Authorize AgentClash CLI</h1>
          <p style={descStyle}>
            The CLI is requesting access to your account.
          </p>
        </div>

        <div style={userInfoStyle}>
          <div style={labelStyle}>Signed in as</div>
          <div style={valueStyle}>{user.email}</div>
          {user.firstName && (
            <div style={{ ...valueStyle, opacity: 0.6, fontSize: "0.8125rem" }}>
              {user.firstName} {user.lastName}
            </div>
          )}
        </div>

        <CLIApproveButton
          port={portNum}
          state={state}
        />
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

const codeStyle: React.CSSProperties = {
  background: "rgba(255, 255, 255, 0.06)",
  padding: "0.15rem 0.4rem",
  borderRadius: "4px",
  fontFamily: "monospace",
  fontSize: "0.8125rem",
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
