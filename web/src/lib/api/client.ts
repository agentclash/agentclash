import { ApiError, NetworkError } from "./errors";
import type { ApiErrorResponse } from "./types";

/** Pagination envelope returned by all list endpoints. */
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface ApiResponse<T> {
  data: T;
  status: number;
  headers: Headers;
}

/** Options shared by every request method. */
interface RequestOptions {
  /** Query parameters appended to the URL. */
  params?: Record<string, string | number | undefined>;
  /** Additional headers merged onto the request. */
  headers?: Record<string, string>;
  /** AbortSignal for cancellation. */
  signal?: AbortSignal;
  /** Non-2xx status codes that should be parsed as JSON instead of throwing. */
  allowedStatuses?: number[];
}

function resolveBaseUrl(): string {
  // Server-side: prefer API_URL (not exposed to browser).
  // Client-side: fall back to NEXT_PUBLIC_API_URL.
  const url =
    typeof window === "undefined"
      ? process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL
      : process.env.NEXT_PUBLIC_API_URL;

  if (!url) {
    throw new Error(
      "Missing API_URL or NEXT_PUBLIC_API_URL environment variable",
    );
  }
  return url.replace(/\/+$/, ""); // strip trailing slash
}

function buildUrl(
  path: string,
  params?: Record<string, string | number | undefined>,
): string {
  const base = resolveBaseUrl();
  const url = new URL(path, base);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value));
    }
  }
  return url.toString();
}

async function parseErrorResponse(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as ApiErrorResponse;
    return new ApiError(res.status, body.error.code, body.error.message);
  } catch {
    return new ApiError(res.status, "unknown", res.statusText || "Request failed");
  }
}

async function requestWithMeta<T>(
  method: string,
  path: string,
  token: string | undefined,
  body: unknown | undefined,
  opts: RequestOptions = {},
): Promise<ApiResponse<T>> {
  const url = buildUrl(path, opts.params);

  const headers: Record<string, string> = {
    Accept: "application/json",
    ...opts.headers,
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  if (body !== undefined && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }

  let res: Response;
  try {
    res = await fetch(url, {
      method,
      headers,
      body:
        body !== undefined
          ? typeof body === "string"
            ? body
            : JSON.stringify(body)
          : undefined,
      signal: opts.signal,
    });
  } catch (err) {
    throw new NetworkError(
      err instanceof Error ? err.message : "Network request failed",
    );
  }

  if (!res.ok && !opts.allowedStatuses?.includes(res.status)) {
    throw await parseErrorResponse(res);
  }

  // 204 No Content — return undefined as T
  if (res.status === 204) {
    return {
      data: undefined as T,
      status: res.status,
      headers: res.headers,
    };
  }

  return {
    data: (await res.json()) as T,
    status: res.status,
    headers: res.headers,
  };
}

async function request<T>(
  method: string,
  path: string,
  token: string | undefined,
  body: unknown | undefined,
  opts: RequestOptions = {},
): Promise<T> {
  const response = await requestWithMeta<T>(method, path, token, body, opts);
  return response.data;
}

/**
 * Create a typed API client bound to an access token.
 *
 * Server-side usage:
 *   const { accessToken } = await withAuth();
 *   const api = createApiClient(accessToken);
 *   const session = await api.get<SessionResponse>("/v1/auth/session");
 *
 * Client-side usage (via useAccessToken):
 *   const { getAccessToken } = useAccessToken();
 *   const token = await getAccessToken();
 *   const api = createApiClient(token);
 */
export function createApiClient(token?: string) {
  return {
    get<T>(path: string, opts?: RequestOptions): Promise<T> {
      return request<T>("GET", path, token, undefined, opts);
    },

    post<T>(path: string, body?: unknown, opts?: RequestOptions): Promise<T> {
      return request<T>("POST", path, token, body, opts);
    },

    postWithMeta<T>(
      path: string,
      body?: unknown,
      opts?: RequestOptions,
    ): Promise<ApiResponse<T>> {
      return requestWithMeta<T>("POST", path, token, body, opts);
    },

    /** POST with a raw string body and explicit content type. */
    postRaw<T>(
      path: string,
      body: string,
      contentType: string,
      opts?: RequestOptions,
    ): Promise<T> {
      return request<T>("POST", path, token, body, {
        ...opts,
        headers: { ...opts?.headers, "Content-Type": contentType },
      });
    },

    put<T>(path: string, body?: unknown, opts?: RequestOptions): Promise<T> {
      return request<T>("PUT", path, token, body, opts);
    },

    patch<T>(path: string, body?: unknown, opts?: RequestOptions): Promise<T> {
      return request<T>("PATCH", path, token, body, opts);
    },

    del<T>(path: string, opts?: RequestOptions): Promise<T> {
      return request<T>("DELETE", path, token, undefined, opts);
    },

    /**
     * Fetch a paginated list endpoint.
     * Automatically appends `limit` and `offset` as query params.
     */
    paginated<T>(
      path: string,
      opts?: RequestOptions & { limit?: number; offset?: number },
    ): Promise<PaginatedResponse<T>> {
      const { limit, offset, ...rest } = opts ?? {};
      return request<PaginatedResponse<T>>("GET", path, token, undefined, {
        ...rest,
        params: {
          ...rest.params,
          limit: limit ?? 20,
          offset: offset ?? 0,
        },
      });
    },
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;
