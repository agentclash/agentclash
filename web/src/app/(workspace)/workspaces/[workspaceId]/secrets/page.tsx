import { SecretsClient } from "./secrets-client";

export default async function SecretsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <SecretsClient workspaceId={workspaceId} />;
}
