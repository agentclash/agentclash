import { ProviderAccountsClient } from "./provider-accounts-client";

export default async function ProviderAccountsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <ProviderAccountsClient workspaceId={workspaceId} />;
}
