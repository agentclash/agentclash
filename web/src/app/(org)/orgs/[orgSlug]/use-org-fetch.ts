"use client";

import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";

interface UseOrgFetchResult<T> {
  data: T | null;
  total: number;
  loading: boolean;
}

/**
 * Generic hook for fetching paginated org data.
 * Returns the first page of items + total count.
 */
export function useOrgFetch<T>(
  path: string,
  pageSize = 50,
): UseOrgFetchResult<T[]> {
  const { getAccessToken } = useAccessToken();
  const [data, setData] = useState<T[] | null>(null);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        if (!token) return;
        const api = createApiClient(token);
        const res = await api.get<{ items: T[]; total: number }>(path, {
          params: { limit: pageSize, offset: 0 },
        });
        if (!cancelled) {
          setData(res.items);
          setTotal(res.total);
        }
      } catch {
        if (!cancelled) setData([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getAccessToken, path, pageSize]);

  return { data, total, loading };
}
