import { describe, expect, it } from "vitest";
import { CURATED_REPLAY_SHARE_URL } from "./curated-replay";

describe("CURATED_REPLAY_SHARE_URL", () => {
  it("starts unset -- no share token can be minted from a PR (see #1242)", () => {
    // If this ever fails because someone filled in a real token, that's
    // expected and this assertion should be updated/removed at the same
    // time the landing page link is verified against production.
    expect(CURATED_REPLAY_SHARE_URL).toBeNull();
  });

  it("is shaped like a /share/ path whenever it is set", () => {
    if (CURATED_REPLAY_SHARE_URL === null) return;
    expect(CURATED_REPLAY_SHARE_URL).toMatch(/\/share\//);
  });
});
