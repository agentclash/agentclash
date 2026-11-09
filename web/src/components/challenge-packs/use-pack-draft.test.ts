import { describe, expect, it, vi } from "vitest";

import type { CompileDraftResponse, Composition } from "./lib/types";
import {
  packDraftReducer,
  publishDisabled,
  saveStatusLabel,
  type State,
} from "./use-pack-draft";

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: vi.fn(async () => "token") }),
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

function initial(): State {
  return {
    composition: {} as Composition,
    selection: { section: "overview" },
    save: { status: "idle" },
    compiling: false,
    compile: null,
    publishing: false,
  };
}

const validCompile = { valid: true, errors: [] } as unknown as CompileDraftResponse;

describe("packDraftReducer save lifecycle", () => {
  it("shows Saved after a successful save", () => {
    let state = packDraftReducer(initial(), { type: "savingStart" });
    expect(saveStatusLabel(state)).toBe("Saving…");
    state = packDraftReducer(state, { type: "savingDone" });
    expect(state.save.status).toBe("saved");
    expect(saveStatusLabel(state)).toBe("Saved");
  });

  it("does not show Saved after a failed save", () => {
    let state = packDraftReducer(initial(), { type: "savingStart" });
    state = packDraftReducer(state, { type: "savingFailed" });
    expect(state.save.status).toBe("failed");
    expect(saveStatusLabel(state)).toBe("Not saved");
  });

  it("drops an earlier Saved when a later save fails", () => {
    let state = packDraftReducer(initial(), { type: "savingDone" });
    state = packDraftReducer(state, { type: "savingStart" });
    state = packDraftReducer(state, { type: "savingFailed" });
    expect(saveStatusLabel(state)).toBe("Not saved");
  });
});

describe("publishDisabled", () => {
  it("allows publish when idle with a valid compile", () => {
    expect(publishDisabled({ ...initial(), compile: validCompile })).toBe(false);
  });

  it("blocks publish while saving or compiling", () => {
    const base = { ...initial(), compile: validCompile };
    expect(publishDisabled({ ...base, save: { status: "saving" } })).toBe(true);
    expect(publishDisabled({ ...base, compiling: true })).toBe(true);
  });

  it("blocks publish while publishing or when the draft is invalid", () => {
    expect(publishDisabled({ ...initial(), publishing: true })).toBe(true);
    expect(
      publishDisabled({
        ...initial(),
        compile: { ...validCompile, valid: false },
      }),
    ).toBe(true);
  });
});
