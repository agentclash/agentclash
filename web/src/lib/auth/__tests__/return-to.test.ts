import { describe, expect, it } from "vitest";
import {
  buildDeviceReturnTo,
  normalizeDeviceUserCode,
  sanitizeReturnTo,
} from "../return-to";

describe("sanitizeReturnTo", () => {
  it("defaults to dashboard for unsafe values", () => {
    expect(sanitizeReturnTo(undefined)).toBe("/dashboard");
    expect(sanitizeReturnTo("https://evil.example")).toBe("/dashboard");
    expect(sanitizeReturnTo("//evil.example")).toBe("/dashboard");
    expect(sanitizeReturnTo("/workspaces/ws-1")).toBe("/dashboard");
  });

  it("preserves the device verification route with a normalized code", () => {
    expect(sanitizeReturnTo("/auth/device?user_code=ab cd-1234&foo=bar")).toBe(
      "/auth/device?user_code=ABCD-1234",
    );
  });
});

describe("device return-to helpers", () => {
  it("normalizes the verification code format", () => {
    expect(normalizeDeviceUserCode("ab cd-1234")).toBe("ABCD-1234");
    expect(normalizeDeviceUserCode("abc")).toBe("ABC");
    expect(normalizeDeviceUserCode(null)).toBe("");
  });

  it("builds a safe device return path", () => {
    expect(buildDeviceReturnTo("ab cd-1234")).toBe(
      "/auth/device?user_code=ABCD-1234",
    );
    expect(buildDeviceReturnTo("")).toBe("/auth/device");
  });
});
