"use client";

import { useState } from "react";
import { useFormStatus } from "react-dom";
import { normalizeDeviceUserCode, buildDeviceReturnTo } from "@/lib/auth/return-to";
import { SignInButton } from "../login/sign-in-button";

interface DeviceCodeFormProps {
  approved?: boolean;
  initialUserCode: string;
  isAuthenticated: boolean;
  approveAction: (formData: FormData) => void | Promise<void>;
}

function hasCompleteCode(userCode: string): boolean {
  return userCode.replace(/[^A-Z0-9]/g, "").length === 8;
}

function ApproveButton({ disabled }: { disabled: boolean }) {
  const { pending } = useFormStatus();

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
            : "rgba(255, 255, 255, 0.9)",
        color: disabled || pending ? "rgba(255, 255, 255, 0.45)" : "#060606",
        borderRadius: "8px",
        fontWeight: 600,
        fontSize: "0.95rem",
        textAlign: "center",
        border: "none",
        cursor: disabled || pending ? "not-allowed" : "pointer",
      }}
    >
      {pending ? "Approving..." : "Approve CLI Login"}
    </button>
  );
}

export function DeviceCodeForm({
  approved = false,
  initialUserCode,
  isAuthenticated,
  approveAction,
}: DeviceCodeFormProps) {
  const [userCode, setUserCode] = useState(initialUserCode);
  const normalizedCode = normalizeDeviceUserCode(userCode);
  const isComplete = hasCompleteCode(normalizedCode);

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
        disabled={approved}
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
        Paste the code from your terminal. The CLI will finish login after you
        approve this request here.
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
      ) : isAuthenticated ? (
        <form action={approveAction}>
          <input type="hidden" name="user_code" value={normalizedCode} />
          <ApproveButton disabled={!isComplete} />
        </form>
      ) : (
        <SignInButton
          label="Sign In To Continue"
          returnTo={buildDeviceReturnTo(normalizedCode)}
        />
      )}
    </div>
  );
}
