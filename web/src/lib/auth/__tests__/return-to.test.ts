import { describe, expect, it } from "vitest";
import {
  buildInviteReturnTo,
  buildGitHubSetupReturnTo,
  buildDashboardReturnTo,
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

  it("preserves paid plan intent on the dashboard return path", () => {
    expect(sanitizeReturnTo("/dashboard?plan=pro&evil=https://example.com")).toBe(
      "/dashboard?plan=pro",
    );
    expect(sanitizeReturnTo("/dashboard?plan=enterprise")).toBe("/dashboard");
  });

  it("preserves invite accept routes", () => {
    const membershipId = "018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";
    const inviteToken = "invite_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi0123456789";

    expect(sanitizeReturnTo(`/invites/organization/${membershipId}`)).toBe(
      `/invites/organization/${membershipId}`,
    );
    expect(sanitizeReturnTo(`/invites/workspace/${inviteToken}`)).toBe(
      `/invites/workspace/${inviteToken}`,
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

describe("dashboard return-to helpers", () => {
  it("keeps only supported paid plan intent", () => {
    expect(buildDashboardReturnTo(new URLSearchParams("plan=team"))).toBe(
      "/dashboard?plan=team",
    );
    expect(buildDashboardReturnTo(new URLSearchParams("plan=free"))).toBe(
      "/dashboard",
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
    expect(
      buildInviteReturnTo(
        "workspace",
        "/invites/workspace/invite_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi0123456789",
      ),
    ).toBe(
      "/invites/workspace/invite_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi0123456789",
    );
  });
});
