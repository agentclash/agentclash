// @vitest-environment node
import { expect, it } from "vitest";
import { getBundledTryCliDemos } from "./catalog.server";
import { TRY_CLI_DEMO_SLUGS } from "./demo-slugs";
it("auth demo identities exactly match the bundled catalog", () => {
  expect([...TRY_CLI_DEMO_SLUGS].sort()).toEqual(getBundledTryCliDemos().map(demo => demo.slug).sort());
});
