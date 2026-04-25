"use client";

import type { ReactNode } from "react";
import { AuthKitProvider, useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { SWRConfig } from "swr";
import { createSWRApiFetcher } from "@/lib/api/swr";

function WorkspaceDataProvider({ children }: { children: ReactNode }) {
  const { getAccessToken } = useAccessToken();

  return (
    <SWRConfig
      value={{
        fetcher: createSWRApiFetcher(getAccessToken),
        keepPreviousData: true,
        revalidateOnFocus: false,
        revalidateOnReconnect: true,
        dedupingInterval: 2_000,
        shouldRetryOnError: false,
      }}
    >
      {children}
    </SWRConfig>
  );
}

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <AuthKitProvider>
      <WorkspaceDataProvider>{children}</WorkspaceDataProvider>
    </AuthKitProvider>
  );
}
