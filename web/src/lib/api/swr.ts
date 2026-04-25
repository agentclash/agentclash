import { useCallback } from "react";
import useSWR, { type SWRConfiguration, type SWRResponse, useSWRConfig } from "swr";
import { createApiClient } from "@/lib/api/client";

export type ApiQueryParams = Record<string, string | number | undefined>;
export type ApiQueryKey =
  | readonly [path: string]
  | readonly [path: string, params: ApiQueryParams];

export function apiQueryKey(
  path: string,
  params?: ApiQueryParams,
): ApiQueryKey {
  if (!params || Object.keys(params).length === 0) {
    return [path];
  }
  return [path, params];
}

export function createSWRApiFetcher(
  getAccessToken: () => Promise<string | null | undefined>,
) {
  return async function swrApiFetcher<T>(key: string | ApiQueryKey): Promise<T> {
    const [path, params] = typeof key === "string" ? [key, undefined] : key;
    const token = await getAccessToken();
    const api = createApiClient(token ?? undefined);
    return api.get<T>(path, params ? { params } : undefined);
  };
}

export function useApiQuery<T>(
  path: string,
  params?: ApiQueryParams,
  config?: SWRConfiguration<T>,
): SWRResponse<T> {
  return useSWR<T>(apiQueryKey(path, params), config);
}

export function useApiListQuery<T>(
  path: string,
  params?: ApiQueryParams,
  config?: SWRConfiguration<{ items: T[] }>,
): SWRResponse<{ items: T[] }> {
  return useSWR<{ items: T[] }>(apiQueryKey(path, params), config);
}

export function usePaginatedApiQuery<T>(
  path: string,
  params?: ApiQueryParams,
  config?: SWRConfiguration<{
    items: T[];
    total: number;
    limit: number;
    offset: number;
  }>,
): SWRResponse<{
  items: T[];
  total: number;
  limit: number;
  offset: number;
}> {
  return useSWR<{
    items: T[];
    total: number;
    limit: number;
    offset: number;
  }>(apiQueryKey(path, params), config);
}

export function useApiMutator() {
  const { mutate } = useSWRConfig();

  const mutateMany = useCallback(async (keys: ApiQueryKey[]) => {
    await Promise.all(keys.map((key) => mutate(key)));
  }, [mutate]);

  return { mutate, mutateMany };
}
