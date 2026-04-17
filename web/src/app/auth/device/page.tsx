import { withAuth } from "@workos-inc/authkit-nextjs";
import { buildDeviceReturnTo, normalizeDeviceUserCode } from "@/lib/auth/return-to";
import { approveDeviceCodeAction, denyDeviceCodeAction } from "./actions";
import { DeviceCodeForm } from "./device-code-form";

function getErrorMessage(errorCode: string | undefined): string | null {
  switch (errorCode) {
    case "missing_code":
      return "Enter the verification code from your terminal to continue.";
    case "invalid_code":
      return "That verification code is invalid or has already been used.";
    case "expired_code":
      return "That verification code expired. Run agentclash auth login again.";
    case "auth_required":
      return "Sign in again before approving this CLI login.";
    case "request_failed":
      return "We could not approve this CLI login. Try again.";
    default:
      return null;
  }
}

function StatusMessage({
  tone,
  message,
}: {
  tone: "error" | "success" | "warning";
  message: string;
}) {
  const isError = tone === "error";
  const isWarning = tone === "warning";

  return (
    <div
      style={{
        borderRadius: "10px",
        border: isError
          ? "1px solid rgba(255, 107, 107, 0.28)"
          : isWarning
            ? "1px solid rgba(255, 186, 73, 0.28)"
            : "1px solid rgba(90, 181, 117, 0.28)",
        background: isError
          ? "rgba(255, 107, 107, 0.1)"
          : isWarning
            ? "rgba(255, 186, 73, 0.1)"
            : "rgba(90, 181, 117, 0.12)",
        color: isError
          ? "rgba(255, 214, 214, 0.96)"
          : isWarning
            ? "rgba(255, 233, 191, 0.95)"
            : "rgba(211, 255, 223, 0.95)",
        padding: "0.9rem 1rem",
        fontSize: "0.95rem",
        marginBottom: "1rem",
      }}
    >
      {message}
    </div>
  );
}

export default async function DevicePage({
  searchParams,
}: {
  searchParams: Promise<{
    error?: string;
    status?: string;
    user_code?: string;
  }>;
}) {
  const [{ user }, params] = await Promise.all([withAuth(), searchParams]);
  const userCode = normalizeDeviceUserCode(params.user_code);
  const returnTo = buildDeviceReturnTo(userCode);
  const errorMessage = getErrorMessage(params.error);
  const approved = params.status === "approved";
  const denied = params.status === "denied";
  const successMessage = approved
    ? "CLI login approved. Return to the terminal to finish signing in."
    : null;
  const deniedMessage = denied
    ? "CLI login cancelled. Return to the terminal to finish."
    : null;
  const signedInAs = user?.email ?? user?.firstName ?? "your AgentClash account";

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
      <div style={{ width: "100%", maxWidth: "520px" }}>
        <div style={{ textAlign: "center", marginBottom: "2rem" }}>
          <p
            style={{
              color: "rgba(255, 255, 255, 0.45)",
              fontSize: "0.8rem",
              letterSpacing: "0.12em",
              textTransform: "uppercase",
              marginBottom: "0.75rem",
            }}
          >
            AgentClash CLI Verification
          </p>
          <h1
            style={{
              fontFamily: "var(--font-display), serif",
              fontSize: "2.1rem",
              letterSpacing: "-0.03em",
              color: "rgba(255, 255, 255, 0.96)",
              margin: 0,
            }}
          >
            Verify this CLI login
          </h1>
          <p
            style={{
              color: "rgba(255, 255, 255, 0.55)",
              fontSize: "0.95rem",
              lineHeight: 1.6,
              marginTop: "0.85rem",
            }}
          >
            Sign in with your AgentClash account, then approve the pending CLI
            login from this page. The CLI token never passes through the browser
            URL.
          </p>
        </div>

        <div
          style={{
            background: "rgba(255, 255, 255, 0.03)",
            border: "1px solid rgba(255, 255, 255, 0.08)",
            borderRadius: "16px",
            padding: "2rem",
            boxShadow: "0 24px 80px rgba(0, 0, 0, 0.22)",
          }}
        >
          {errorMessage ? <StatusMessage tone="error" message={errorMessage} /> : null}
          {successMessage ? (
            <StatusMessage tone="success" message={successMessage} />
          ) : null}
          {deniedMessage ? (
            <StatusMessage tone="warning" message={deniedMessage} />
          ) : null}

          <div
            style={{
              borderRadius: "10px",
              background: "rgba(255, 255, 255, 0.04)",
              border: "1px solid rgba(255, 255, 255, 0.08)",
              padding: "0.9rem 1rem",
              marginBottom: "1.5rem",
            }}
          >
            <div
              style={{
                color: "rgba(255, 255, 255, 0.45)",
                fontSize: "0.78rem",
                textTransform: "uppercase",
                letterSpacing: "0.08em",
                marginBottom: "0.35rem",
              }}
            >
              {user ? "Signed in as" : "Next step"}
            </div>
            <div
              style={{
                color: "rgba(255, 255, 255, 0.92)",
                fontSize: "0.96rem",
              }}
            >
              {user
                ? signedInAs
                : "Sign in first, then approve the CLI login request."}
            </div>
          </div>

          <DeviceCodeForm
            approved={approved}
            denied={denied}
            initialUserCode={userCode}
            isAuthenticated={Boolean(user)}
            approveAction={approveDeviceCodeAction}
            denyAction={denyDeviceCodeAction}
          />

          {!approved && !denied ? (
            <p
              style={{
                color: "rgba(255, 255, 255, 0.38)",
                fontSize: "0.8rem",
                marginTop: "1rem",
                marginBottom: 0,
              }}
            >
              If your code is missing, go back to the terminal and rerun{" "}
              <code
                style={{
                  background: "rgba(255, 255, 255, 0.06)",
                  padding: "0.15rem 0.35rem",
                  borderRadius: "6px",
                }}
              >
                agentclash auth login
              </code>
              .
            </p>
          ) : null}
        </div>

        {!user && returnTo !== "/auth/device" ? (
          <p
            style={{
              color: "rgba(255, 255, 255, 0.35)",
              fontSize: "0.8rem",
              marginTop: "1rem",
              textAlign: "center",
            }}
          >
            After sign-in, you will return here with the same verification code.
          </p>
        ) : null}
      </div>
    </div>
  );
}
