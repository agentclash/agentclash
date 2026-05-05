import { describe, expect, it } from "vitest";
import {
  buildInviteReturnTo,
  buildGitHubSetupReturnTo,
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

  it("preserves a sanitized GitHub setup callback", () => {
    expect(
      sanitizeReturnTo(
        "/github/setup?installation_id=42&state=abc_DEF-123.sig_456&setup_action=install&evil=https://example.com",
      ),
    ).toBe(
      "/github/setup?installation_id=42&state=abc_DEF-123.sig_456&setup_action=install",
    );
  });

  it("preserves invite accept routes", () => {
    const membershipId = "018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";

    expect(sanitizeReturnTo(`/invites/organization/${membershipId}`)).toBe(
      `/invites/organization/${membershipId}`,
    );
    expect(sanitizeReturnTo(`/invites/workspace/${membershipId}`)).toBe(
      `/invites/workspace/${membershipId}`,
    );
  });

  it("rejects malformed invite accept routes", () => {
    expect(sanitizeReturnTo("/invites/organization/not-a-uuid")).toBe(
      "/dashboard",
    );
    expect(
      sanitizeReturnTo(
        "/invites/workspace/018f9f2e-65cb-7d0b-9c98-0df2ce4076d8/extra",
      ),
    ).toBe("/dashboard");
  });
});

describe("github setup return-to helpers", () => {
  it("requires installation id and signed state", () => {
    expect(buildGitHubSetupReturnTo(new URLSearchParams("installation_id=x"))).toBe(
      "/github/setup",
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

describe("invite return-to helpers", () => {
  it("builds safe invite return paths", () => {
    const membershipId = "018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";

    expect(
      buildInviteReturnTo("organization", `/invites/organization/${membershipId}`),
    ).toBe(`/invites/organization/${membershipId}`);
    expect(buildInviteReturnTo("workspace", "/invites/workspace/nope")).toBe(
      "/dashboard",
    );
  });
});
