import { beforeEach, describe, expect, it } from "vitest";
import { readBuildDrafts, writeBuildDrafts } from "./vibe-build-drafts";

describe("unsent Build drafts", () => {
  beforeEach(() => sessionStorage.clear());
  it("restores guide and trial text without copying another evaluation's input", () => {
    writeBuildDrafts("a", "change the wording", { "a:v1:thread": "try this email", "b:v2:thread": "private in b" });
    expect(readBuildDrafts("a")).toEqual({ version: 1, guide: "change the wording", trials: { "a:v1:thread": "try this email" } });
    expect(readBuildDrafts("b")).toBeUndefined();
  });
  it("ignores corrupted or foreign draft data and restores an intentionally empty draft", () => {
    sessionStorage.setItem("vibe-build-drafts:a", "broken json");
    expect(readBuildDrafts("a")).toBeUndefined();
    sessionStorage.setItem("vibe-build-drafts:a", JSON.stringify({ version: 1, guide: "", trials: { "b:v1:t": "wrong context", "a:v1:t": "" } }));
    expect(readBuildDrafts("a")).toEqual({ version: 1, guide: "", trials: { "a:v1:t": "" } });
  });
});
