import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SignInButton } from "./sign-in-button";
import { signInAction, signUpAction } from "./actions";

vi.mock("./actions", () => ({
  signInAction: vi.fn(),
  signUpAction: vi.fn(),
}));

let root: Root | null = null;
let container: HTMLDivElement | null = null;

function render(element: React.ReactElement) {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => {
    root?.render(element);
  });
}

async function submitForm() {
  const form = container?.querySelector("form");
  await act(async () => {
    form?.requestSubmit();
  });
}

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
  vi.clearAllMocks();
});

describe("SignInButton", () => {
  it("brands the hosted auth handoff as AgentClash", () => {
    render(<SignInButton />);

    expect(container?.textContent).toContain("Continue with AgentClash");
    expect(container?.textContent).not.toContain("Sign in with WorkOS");
  });

  it("preserves the sanitized return target in the form", () => {
    render(<SignInButton returnTo="/auth/device?user_code=ABCD-1234" />);

    const input = container?.querySelector<HTMLInputElement>(
      'input[name="returnTo"]',
    );
    expect(input?.value).toBe("/auth/device?user_code=ABCD-1234");
  });

  it("dispatches the sign-in action in the default mode", async () => {
    render(<SignInButton returnTo="/dashboard" />);

    await submitForm();

    expect(signInAction).toHaveBeenCalledTimes(1);
    expect(signUpAction).not.toHaveBeenCalled();
  });

  it("dispatches the sign-up action (with shared branding) in sign-up mode", async () => {
    render(<SignInButton mode="signup" returnTo="/dashboard" />);

    expect(container?.querySelector("form")).not.toBeNull();
    expect(container?.textContent).toContain("Continue with AgentClash");

    await submitForm();

    expect(signUpAction).toHaveBeenCalledTimes(1);
    expect(signInAction).not.toHaveBeenCalled();
  });
});
