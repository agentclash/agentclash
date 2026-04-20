import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { VersionEditor } from "./version-editor";

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const {
  mockRefresh,
  mockGetAccessToken,
  mockCreateApiClient,
  toast,
} = vi.hoisted(() => ({
  mockRefresh: vi.fn(),
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: mockRefresh }),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("sonner", () => ({
  toast,
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
    React.createElement("button", props, children),
}));

vi.mock("@/components/ui/badge", () => ({
  Badge: ({ children }: { children: React.ReactNode }) =>
    React.createElement("span", null, children),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) =>
    React.createElement("input", props),
}));

vi.mock("@/components/ui/json-field", () => ({
  JsonField: ({
    label,
    value,
    onChange,
  }: {
    label: string;
    value: string;
    onChange: (value: string) => void;
  }) =>
    React.createElement("textarea", {
      "data-json-label": label,
      value,
      onChange: (event: React.ChangeEvent<HTMLTextAreaElement>) =>
        onChange(event.target.value),
    }),
}));

vi.mock("@/components/ui/tabs", async () => {
  const React = await import("react");
  const TabsValueContext = React.createContext("guided");
  const TabsChangeContext = React.createContext<(value: string) => void>(() => {});

  return {
    Tabs: ({
      value,
      onValueChange,
      children,
    }: {
      value: string;
      onValueChange: (value: string) => void;
      children: React.ReactNode;
    }) =>
      React.createElement(
        TabsValueContext.Provider,
        { value },
        React.createElement(
          TabsChangeContext.Provider,
          { value: onValueChange },
          children,
        ),
      ),
    TabsList: ({ children }: { children: React.ReactNode }) =>
      React.createElement("div", null, children),
    TabsTrigger: ({
      value,
      children,
    }: {
      value: string;
      children: React.ReactNode;
    }) => {
      const setValue = React.useContext(TabsChangeContext);
      return React.createElement(
        "button",
        { onClick: () => setValue(value) },
        children,
      );
    },
    TabsContent: ({
      value,
      children,
    }: {
      value: string;
      children: React.ReactNode;
    }) => {
      const active = React.useContext(TabsValueContext);
      return active === value ? React.createElement("div", null, children) : null;
    },
  };
});

vi.mock("lucide-react", () => ({
  CheckCircle: () => React.createElement("span", null, "ready"),
  Loader2: () => React.createElement("span", null, "loader"),
  Save: () => React.createElement("span", null, "save"),
  ShieldCheck: () => React.createElement("span", null, "validate"),
  Sparkles: () => React.createElement("span", null, "sparkles"),
}));

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

function clickElement(element: Element) {
  element.dispatchEvent(
    new MouseEvent("click", {
      bubbles: true,
      cancelable: true,
    }),
  );
}

function changeInput(
  element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement,
  value: string,
) {
  const prototype = Object.getPrototypeOf(element) as {
    value?: PropertyDescriptor;
  };
  const descriptor =
    Object.getOwnPropertyDescriptor(prototype, "value") ??
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value") ??
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value") ??
    Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value");
  descriptor?.set?.call(element, value);
  element.dispatchEvent(new window.Event("input", { bubbles: true, cancelable: true }));
  element.dispatchEvent(new window.Event("change", { bubbles: true, cancelable: true }));
}

function findButton(text: string) {
  const button = Array.from(document.querySelectorAll("button")).find((candidate) =>
    candidate.textContent?.includes(text),
  );
  if (!button) throw new Error(`Button with text ${text} not found`);
  return button;
}

function findTextAreaByLabel(label: string) {
  const labels = Array.from(document.querySelectorAll("label"));
  const labelNode = labels.find((candidate) =>
    candidate.textContent?.includes(label),
  );
  if (!labelNode) throw new Error(`Label ${label} not found`);
  const textArea = labelNode.parentElement?.querySelector("textarea");
  if (!(textArea instanceof HTMLTextAreaElement)) {
    throw new Error(`Textarea for ${label} not found`);
  }
  return textArea;
}

function findInputByLabel(label: string) {
  const labels = Array.from(document.querySelectorAll("label"));
  const labelNode = labels.find((candidate) =>
    candidate.textContent?.includes(label),
  );
  if (!labelNode) throw new Error(`Label ${label} not found`);
  const input = labelNode.parentElement?.querySelector("input");
  if (!(input instanceof HTMLInputElement)) {
    throw new Error(`Input for ${label} not found`);
  }
  return input;
}

function findJsonField(label: string) {
  const field = Array.from(
    document.querySelectorAll("textarea[data-json-label]"),
  ).find((candidate) => candidate.getAttribute("data-json-label")?.includes(label));
  if (!(field instanceof HTMLTextAreaElement)) {
    throw new Error(`JSON field ${label} not found`);
  }
  return field;
}

function renderEditor() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);

  act(() => {
    root.render(
      React.createElement(VersionEditor, {
        version: {
          id: "version-1",
          agent_build_id: "build-1",
          version_number: 2,
          version_status: "draft",
          agent_kind: "workflow_agent",
          interface_spec: { primary_input: "support_ticket" },
          policy_spec: {
            role: "You are a triage assistant.",
            instructions: "Classify the issue and recommend the next step.",
            success_conditions: "Return urgency and owner.",
          },
          reasoning_spec: {},
          memory_spec: { strategy: "session" },
          workflow_spec: { tool_strategy: "prefer_tools_first" },
          guardrail_spec: {},
          model_spec: {},
          output_schema: {
            type: "object",
            properties: {
              answer: { type: "string" },
            },
            required: ["answer"],
          },
          trace_contract: {},
          publication_spec: {},
          tools: [],
          knowledge_sources: [],
          created_at: "2026-04-20T00:00:00Z",
        },
      }),
    );
  });

  return {
    cleanup: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe("VersionEditor", () => {
  beforeEach(() => {
    document.body.innerHTML = "";
    mockRefresh.mockReset();
    mockGetAccessToken.mockReset();
    mockCreateApiClient.mockReset();
    toast.success.mockReset();
    toast.error.mockReset();
  });

  it("hydrates guided fields and saves guided edits through the existing patch API", async () => {
    mockGetAccessToken.mockResolvedValue("token");
    const patch = vi.fn().mockResolvedValue(undefined);
    const post = vi.fn().mockResolvedValue({ valid: true, errors: [] });
    mockCreateApiClient.mockReturnValue({ patch, post });

    const { cleanup } = renderEditor();

    expect(findTextAreaByLabel("Role").value).toContain("triage assistant");
    expect(findInputByLabel("Primary Input Field").value).toBe("support_ticket");

    act(() => {
      changeTextArea(
        findTextAreaByLabel("Mission / Instructions"),
        "Investigate the report and assign severity.",
      );
    });
    act(() => {
      changeInput(findInputByLabel("Primary Input Field"), "incident_report");
    });
    await flushPromises();
    act(() => {
      clickElement(findButton("Save Draft"));
    });
    await flushPromises();

    expect(patch).toHaveBeenCalledWith(
      "/v1/agent-build-versions/version-1",
      expect.objectContaining({
        agent_kind: "workflow_agent",
        policy_spec: expect.objectContaining({
          instructions: "Investigate the report and assign severity.",
        }),
        interface_spec: expect.objectContaining({
          primary_input: "incident_report",
        }),
      }),
    );
    expect(toast.success).toHaveBeenCalledWith("Saved");

    cleanup();
  });

  it("keeps guided edits visible in advanced JSON mode", () => {
    const { cleanup } = renderEditor();

    act(() => {
      changeTextArea(
        findTextAreaByLabel("Mission / Instructions"),
        "Summarize the issue and route it to the right team.",
      );
    });
    act(() => {
      clickElement(findButton("Advanced JSON"));
    });

    expect(findJsonField("Policy Spec").value).toContain(
      "Summarize the issue and route it to the right team.",
    );

    cleanup();
  });

  it("saves the current draft before validating", async () => {
    mockGetAccessToken.mockResolvedValue("token");
    const patch = vi.fn().mockResolvedValue(undefined);
    const post = vi.fn().mockResolvedValue({ valid: true, errors: [] });
    mockCreateApiClient.mockReturnValue({ patch, post });

    const { cleanup } = renderEditor();

    act(() => {
      changeTextArea(
        findTextAreaByLabel("Mission / Instructions"),
        "Investigate the request, compare competing options, and produce a recommendation backed by traceable evidence.",
      );
    });
    await flushPromises();
    act(() => {
      clickElement(findButton("Validate"));
    });
    await flushPromises();

    expect(patch).toHaveBeenCalledWith(
      "/v1/agent-build-versions/version-1",
      expect.objectContaining({
        policy_spec: expect.objectContaining({
          instructions:
            "Investigate the request, compare competing options, and produce a recommendation backed by traceable evidence.",
        }),
      }),
    );
    expect(post).toHaveBeenCalledWith(
      "/v1/agent-build-versions/version-1/validate",
    );

    cleanup();
  });
});

function changeTextArea(element: HTMLTextAreaElement, value: string) {
  changeInput(element, value);
}
