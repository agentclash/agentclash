import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { OnboardingWizard } from "./onboarding-wizard";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: vi.fn(async () => "token") }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ post: mocks.post }),
}));
vi.mock("@/components/analytics/setup-step-tracker", () => ({
  SetupStepView: () => null,
  trackSetupStepClick: vi.fn(),
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

function type(input: HTMLInputElement, value: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(
    input,
    value,
  );
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

function pressEnter(input: HTMLInputElement) {
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
}

it("sends one onboarding request when Enter is pressed twice", async () => {
  // The request stays in flight for the whole test.
  mocks.post.mockReturnValue(new Promise(() => {}));
  await act(async () => root.render(<OnboardingWizard />));

  const orgInput = container.querySelector<HTMLInputElement>("input")!;
  await act(async () => type(orgInput, "Acme"));
  await act(async () => pressEnter(orgInput));

  const workspaceInput = container.querySelector<HTMLInputElement>("input")!;
  await act(async () => type(workspaceInput, "Production"));
  await act(async () => {
    pressEnter(workspaceInput);
    pressEnter(workspaceInput);
  });
  await act(async () => pressEnter(workspaceInput));

  expect(mocks.post).toHaveBeenCalledTimes(1);
});
