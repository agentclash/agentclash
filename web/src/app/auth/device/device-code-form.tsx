"use client";

import { useState } from "react";
import { useFormStatus } from "react-dom";
import { normalizeDeviceUserCode, buildDeviceReturnTo } from "@/lib/auth/return-to";
import { SignInButton } from "../login/sign-in-button";

interface DeviceCodeFormProps {
  approved?: boolean;
  denied?: boolean;
  initialUserCode: string;
  isAuthenticated: boolean;
  approveAction: (formData: FormData) => void | Promise<void>;
  denyAction: (formData: FormData) => void | Promise<void>;
}

function hasCompleteCode(userCode: string): boolean {
  return userCode.replace(/[^A-Z0-9]/g, "").length === 8;
}

function ActionButton({
  disabled,
  label,
  pendingLabel,
  tone = "primary",
}: {
  disabled: boolean;
  label: string;
  pendingLabel: string;
  tone?: "primary" | "secondary";
}) {
  const { pending } = useFormStatus();
  const isPrimary = tone === "primary";

  return (
    <button
      type="submit"
      disabled={disabled || pending}
      style={{
        display: "block",
        width: "100%",
        padding: "0.85rem 1rem",
        background:
          disabled || pending
            ? "rgba(255, 255, 255, 0.12)"
            : isPrimary
              ? "rgba(255, 255, 255, 0.9)"
              : "transparent",
        color:
          disabled || pending
            ? "rgba(255, 255, 255, 0.45)"
            : isPrimary
              ? "#060606"
              : "rgba(255, 255, 255, 0.72)",
        borderRadius: "8px",
        fontWeight: 600,
        fontSize: "0.95rem",
        textAlign: "center",
        border: isPrimary ? "none" : "1px solid rgba(255, 255, 255, 0.16)",
        cursor: disabled || pending ? "not-allowed" : "pointer",
      }}
    >
      {pending ? pendingLabel : label}
    </button>
  );
}

export function DeviceCodeForm({
  approved = false,
  denied = false,
  initialUserCode,
  isAuthenticated,
  approveAction,
  denyAction,
}: DeviceCodeFormProps) {
  const [userCode, setUserCode] = useState(initialUserCode);
  const normalizedCode = normalizeDeviceUserCode(userCode);
  const isComplete = hasCompleteCode(normalizedCode);
  const completed = approved || denied;

  return (
    <div>
      <label
        htmlFor="device-user-code"
        style={{
          display: "block",
          fontSize: "0.75rem",
          letterSpacing: "0.08em",
          textTransform: "uppercase",
          color: "rgba(255, 255, 255, 0.45)",
          marginBottom: "0.75rem",
        }}
      >
        Verification code
      </label>

      <input
        id="device-user-code"
        type="text"
        inputMode="text"
        autoCapitalize="characters"
        autoCorrect="off"
        spellCheck={false}
        value={userCode}
        onChange={(event) =>
          setUserCode(normalizeDeviceUserCode(event.target.value))
        }
        placeholder="ABCD-EFGH"
        maxLength={9}
        disabled={completed}
        style={{
          display: "block",
          width: "100%",
          padding: "0.85rem 1rem",
          borderRadius: "10px",
          border: "1px solid rgba(255, 255, 255, 0.12)",
          background: "rgba(255, 255, 255, 0.04)",
          color: "rgba(255, 255, 255, 0.96)",
          fontFamily:
            'ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
          fontSize: "1.15rem",
          letterSpacing: "0.14em",
          textTransform: "uppercase",
        }}
      />

      <p
        style={{
          color: "rgba(255, 255, 255, 0.5)",
          fontSize: "0.875rem",
          marginTop: "0.75rem",
          marginBottom: "1.5rem",
        }}
      >
        Only approve this request if the code in your terminal matches exactly.
        The CLI will finish login after you approve it here.
      </p>

      {approved ? (
        <div
          style={{
            borderRadius: "10px",
            border: "1px solid rgba(90, 181, 117, 0.28)",
            background: "rgba(90, 181, 117, 0.12)",
            color: "rgba(211, 255, 223, 0.95)",
            padding: "0.9rem 1rem",
            fontSize: "0.95rem",
          }}
        >
          Approval complete. Return to the terminal and wait for the CLI to
          confirm the session.
        </div>
      ) : denied ? (
        <div
          style={{
            borderRadius: "10px",
            border: "1px solid rgba(255, 186, 73, 0.28)",
            background: "rgba(255, 186, 73, 0.1)",
            color: "rgba(255, 233, 191, 0.95)",
            padding: "0.9rem 1rem",
            fontSize: "0.95rem",
          }}
        >
          Login cancelled. Return to the terminal to finish.
        </div>
      ) : isAuthenticated ? (
        <div style={{ display: "grid", gap: "0.75rem" }}>
          <form action={approveAction}>
            <input type="hidden" name="user_code" value={normalizedCode} />
            <ActionButton
              disabled={!isComplete}
              label="Approve CLI Login"
              pendingLabel="Approving..."
            />
          </form>
          <form action={denyAction}>
            <input type="hidden" name="user_code" value={normalizedCode} />
            <ActionButton
              disabled={!isComplete}
              label="Cancel CLI Login"
              pendingLabel="Cancelling..."
              tone="secondary"
            />
          </form>
        </div>
      ) : (
        <SignInButton
          label="Sign In To Continue"
          returnTo={buildDeviceReturnTo(normalizedCode)}
        />
      )}
    </div>
  );
}
