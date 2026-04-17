import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/errors";
import { buildDeviceStatusPath, mapDeviceActionError } from "./status";

describe("buildDeviceStatusPath", () => {
  it("preserves the normalized device code while adding status fields", () => {
    expect(buildDeviceStatusPath("ab cd-1234", { status: "denied" })).toBe(
      "/auth/device?user_code=ABCD-1234&status=denied",
    );
  });
});

describe("mapDeviceActionError", () => {
  it("maps known backend errors to device page error codes", () => {
    expect(
      mapDeviceActionError(
        new ApiError(400, "invalid_request", "device code expired"),
      ),
    ).toBe("expired_code");
    expect(
      mapDeviceActionError(
        new ApiError(400, "invalid_request", "device code not found or expired"),
      ),
    ).toBe("expired_code");
    expect(
      mapDeviceActionError(
        new ApiError(400, "invalid_request", "user_code is required"),
      ),
    ).toBe("missing_code");
    expect(
      mapDeviceActionError(new ApiError(401, "unauthorized", "auth required")),
    ).toBe("auth_required");
  });

  it("falls back to a retryable request failure", () => {
    expect(mapDeviceActionError(new Error("offline"))).toBe("request_failed");
  });
});
