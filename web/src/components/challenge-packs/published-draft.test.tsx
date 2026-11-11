import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";

import { isDraftEditable, markDraftPublished } from "./lib/draft";
import type { ChallengePackDraft } from "./lib/types";
import { PackBuilder } from "./pack-builder";

const query = vi.hoisted(() => ({
  result: { data: undefined as unknown, error: undefined as unknown, isLoading: false },
}));
vi.mock("@/lib/api/swr", () => ({ useApiQuery: () => query.result }));
vi.mock("./use-pack-draft", () => ({
  PackDraftProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="editor">{children}</div>
  ),
}));
vi.mock("./builder-shell", () => ({ BuilderShell: () => null }));
vi.mock("./vibe-return", () => ({ VibeReturn: () => null }));
vi.mock("./edit-pack-button", () => ({
  EditPackInBuilderButton: ({ versionId }: { versionId: string }) => (
    <button data-testid="edit-as-new-draft" data-version={versionId} />
  ),
}));

function draft(overrides: Partial<ChallengePackDraft> = {}): ChallengePackDraft {
  return {
    id: "draft-1",
    workspace_id: "workspace-1",
    name: "Support pack",
    execution_mode: "native",
    composition: {},
    status: "draft",
    created_at: "2026-11-11T00:00:00Z",
    updated_at: "2026-11-11T00:00:00Z",
    ...overrides,
  };
}

async function renderBuilder(data: ChallengePackDraft) {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  query.result = { data, error: undefined, isLoading: false };
  const node = document.createElement("div");
  document.body.append(node);
  const root = createRoot(node);
  await act(async () => root.render(<PackBuilder workspaceId="workspace-1" draftId={data.id} />));
  return {
    node,
    cleanup: async () => {
      await act(async () => root.unmount());
      node.remove();
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("draft status helpers", () => {
  it("lets only a draft in status draft take edits", () => {
    expect(isDraftEditable({ status: "draft" })).toBe(true);
    expect(isDraftEditable({ status: "published" })).toBe(false);
    expect(isDraftEditable({ status: "discarded" })).toBe(false);
  });

  it("marks a draft published and links it to the new version", () => {
    const published = markDraftPublished(draft(), {
      challenge_pack_id: "pack-1",
      challenge_pack_version_id: "version-1",
    });
    expect(published).toMatchObject({
      status: "published",
      challenge_pack_id: "pack-1",
      last_published_version_id: "version-1",
    });
    expect(isDraftEditable(published)).toBe(false);
  });
});

describe("PackBuilder", () => {
  it("shows the published notice and no editor for a published draft", async () => {
    const view = await renderBuilder(
      draft({ status: "published", challenge_pack_id: "pack-1", last_published_version_id: "version-1" }),
    );
    try {
      expect(view.node.querySelector('[data-testid="editor"]')).toBeNull();
      expect(view.node.textContent).toContain("This draft is published");
      const edit = view.node.querySelector('[data-testid="edit-as-new-draft"]');
      expect(edit?.getAttribute("data-version")).toBe("version-1");
      const open = view.node.querySelector("a");
      expect(open?.getAttribute("href")).toBe("/workspaces/workspace-1/challenge-packs/pack-1");
    } finally {
      await view.cleanup();
    }
  });

  it("offers no new draft when the published version is gone", async () => {
    const view = await renderBuilder(draft({ status: "published", challenge_pack_id: "pack-1" }));
    try {
      expect(view.node.querySelector('[data-testid="edit-as-new-draft"]')).toBeNull();
      expect(view.node.textContent).toContain("no longer available");
    } finally {
      await view.cleanup();
    }
  });

  it("mounts the editor for a draft in status draft", async () => {
    const view = await renderBuilder(draft());
    try {
      expect(view.node.querySelector('[data-testid="editor"]')).not.toBeNull();
      expect(view.node.textContent).not.toContain("This draft is published");
    } finally {
      await view.cleanup();
    }
  });
});
