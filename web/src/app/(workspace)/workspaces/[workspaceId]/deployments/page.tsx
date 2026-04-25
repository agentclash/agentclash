import { DeploymentsClient } from "./deployments-client";

export default async function DeploymentsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <DeploymentsClient workspaceId={workspaceId} />;
}
