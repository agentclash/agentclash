import { withAuth } from "@workos-inc/authkit-nextjs";
import { notFound, redirect } from "next/navigation";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { EvalSessionDetail } from "@/lib/api/types";
import { EvalSessionDetailClient } from "./eval-session-detail-client";

export default async function EvalSessionPage({
  params,
}: {
  params: Promise<{ workspaceId: string; evalSessionId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, evalSessionId } = await params;
  const api = createApiClient(accessToken);

  let detail: EvalSessionDetail;
  try {
    detail = await api.get<EvalSessionDetail>(`/v1/eval-sessions/${evalSessionId}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }

  return (
    <EvalSessionDetailClient
      workspaceId={workspaceId}
      initialDetail={detail}
    />
  );
}
