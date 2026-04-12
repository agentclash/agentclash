/**
 * Typed API error thrown when the backend returns an error envelope.
 * Mirrors: {"error":{"code":"...","message":"..."}}
 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
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
