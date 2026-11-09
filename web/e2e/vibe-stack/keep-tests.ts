import { expect, type Page } from "@playwright/test";

export async function deferInstructionsAndKeepTests(page: Page) {
  const tests = page.getByRole("region", { name: "Your tests", exact: true });
  const save = page.getByRole("dialog").filter({ has: page.getByRole("heading", { name: "Keep these tests", exact: true }) });
  await expect(tests.getByRole("textbox", { name: "Agent instructions", exact: true })).toBeVisible();
  await expect(save).toHaveCount(0);
  await tests.getByRole("button", { name: "Keep for later", exact: true }).click();
  await expect(tests.getByRole("textbox", { name: "Agent instructions", exact: true })).toHaveCount(0);
  await expect(save).toHaveCount(0);
  const keep = tests.getByRole("button", { name: "Keep these tests", exact: true });
  await expect(keep).toBeVisible();
  await keep.click();
  await expect(save).toBeVisible();
}
