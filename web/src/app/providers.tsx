"use client";

import { useMemo, type ReactNode } from "react";
import { AuthKitProvider, useAccessToken } from "@workos-inc/authkit-nextjs/components";
import type { NoUserInfo, UserInfo } from "@workos-inc/authkit-nextjs";
import { SWRConfig } from "swr";
import { createSWRApiFetcher } from "@/lib/api/swr";

export type InitialAuth = Omit<UserInfo | NoUserInfo, "accessToken">;

function WorkspaceDataProvider({ children }: { children: ReactNode }) {
  const { getAccessToken } = useAccessToken();
  const swrConfig = useMemo(
    () => ({
      fetcher: createSWRApiFetcher(getAccessToken),
      keepPreviousData: true,
      revalidateOnFocus: false,
      revalidateOnReconnect: true,
      dedupingInterval: 2_000,
      shouldRetryOnError: false,
    }),
    [getAccessToken],
  );

  return (
    <SWRConfig value={swrConfig}>{children}</SWRConfig>
  );
}

export function AppProviders({ children }: { children: ReactNode }) {
  return <AuthKitProvider>{children}</AuthKitProvider>;
}

export function AuthenticatedAppProviders({
  children,
  initialAuth,
}: {
  children: ReactNode;
  initialAuth: InitialAuth;
}) {
  return (
    <AuthKitProvider initialAuth={initialAuth}>
      <WorkspaceDataProvider>{children}</WorkspaceDataProvider>
    </AuthKitProvider>
  );
}
