import { expect, it } from "vitest";
import { contactEmailLink, VIBE_CONTACT_EMAIL, vibeContact } from "./vibe-contact";

it("always gives an email address, and prefers the address from the API", () => {
  expect(VIBE_CONTACT_EMAIL).toBe("hello@tryagentclash.com");
  expect(vibeContact(undefined)).toEqual({ direct: false, available: false, email: "hello@tryagentclash.com" });
  expect(vibeContact({ direct: false, available: false, email: "" })).toEqual({ direct: false, available: false, email: "hello@tryagentclash.com" });
  expect(vibeContact({ direct: false, available: false, email: "   " })).toEqual({ direct: false, available: false, email: "hello@tryagentclash.com" });
  expect(vibeContact({ available: true, email: " team@example.test " })).toEqual({ direct: false, available: true, email: "team@example.test" });
});

it("encodes the subject and body of a mail link, and refuses a link that is too long", () => {
  const link = contactEmailLink("hello@tryagentclash.com", "Agents for our team", "Returns & refunds\n\nCompany: Northwind");
  expect(link).toBe("mailto:hello%40tryagentclash.com?subject=Agents%20for%20our%20team&body=Returns%20%26%20refunds%0A%0ACompany%3A%20Northwind");
  expect(contactEmailLink("hello@tryagentclash.com", "Agents for our team")).toBe("mailto:hello%40tryagentclash.com?subject=Agents%20for%20our%20team");
  expect(contactEmailLink("hello@tryagentclash.com", "Agents for our team", "x".repeat(2000))).toBeUndefined();
  expect(contactEmailLink("", "Agents for our team", "text")).toBeUndefined();
});

it("enables direct contact only when the API explicitly advertises both capabilities", () => {
  expect(vibeContact({ available: true, direct: true }).direct).toBe(true);
  expect(vibeContact({ available: false, direct: true }).direct).toBe(false);
  expect(vibeContact({ available: true }).direct).toBe(false);
  expect(vibeContact({ available: true, direct: false }).direct).toBe(false);
});
