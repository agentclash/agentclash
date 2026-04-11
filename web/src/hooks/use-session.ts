"use client";

import { useCallback, useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError, NetworkError } from "@/lib/api/errors";
import type { SessionResponse } from "@/lib/api/types";

interface UseSessionReturn {
  session: SessionResponse | null;
  loading: boolean;
  error: Error | null;
  refresh: () => Promise<void>;
}

export function useSession(): UseSessionReturn {
  const { getAccessToken } = useAccessToken();
  const [session, setSession] = useState<SessionResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchSession = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const token = await getAccessToken();
      const api = createApiClient(token);
      const data = await api.get<SessionResponse>("/v1/auth/session");
      setSession(data);
    } catch (err) {
      if (err instanceof ApiError || err instanceof NetworkError) {
        setError(err);
      } else {
        setError(err instanceof Error ? err : new Error("Failed to fetch session"));
      }
    } finally {
      setLoading(false);
    }
  }, [getAccessToken]);

  useEffect(() => {
    fetchSession();
  }, [fetchSession]);

  return { session, loading, error, refresh: fetchSession };
}
