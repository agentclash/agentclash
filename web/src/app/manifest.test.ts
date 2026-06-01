import { describe, expect, it } from "vitest";
import manifest from "./manifest";

describe("manifest", () => {
  it("uses brand colors and the existing android-chrome icons", () => {
    const result = manifest();

    expect(result.name).toContain("AgentClash");
    expect(result.short_name).toBe("AgentClash");
    expect(result.display).toBe("standalone");
    expect(result.theme_color).toBe("#060606");
    expect(result.background_color).toBe("#060606");
    expect(result.icons).toEqual([
      {
        src: "/android-chrome-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/android-chrome-512x512.png",
        sizes: "512x512",
        type: "image/png",
      },
    ]);
  });
});
