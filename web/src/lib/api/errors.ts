/**
 * Typed API error thrown when the backend returns an error envelope.
 * Mirrors: {"error":{"code":"...","message":"..."}}
 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly planKey?: string;
  readonly upgradeTarget?: string;
  readonly limit?: number | null;
  readonly used?: number;
  readonly remaining?: number | null;
  readonly resetAt?: string;
  readonly expiresAt?: string;

  constructor(
    status: number,
    code: string,
    message: string,
    details: {
      planKey?: string;
      upgradeTarget?: string;
      limit?: number | null;
      used?: number;
      remaining?: number | null;
      resetAt?: string;
      expiresAt?: string;
    } = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.planKey = details.planKey;
    this.upgradeTarget = details.upgradeTarget;
    this.limit = details.limit;
    this.used = details.used;
    this.remaining = details.remaining;
    this.resetAt = details.resetAt;
    this.expiresAt = details.expiresAt;
  }
}

/**
 * Thrown when a network-level failure prevents the request from completing.
 */
export class NetworkError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NetworkError";
  }
}
