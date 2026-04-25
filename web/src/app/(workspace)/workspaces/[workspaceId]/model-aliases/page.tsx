import { ModelAliasesClient } from "./model-aliases-client";

export default async function ModelAliasesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <ModelAliasesClient workspaceId={workspaceId} />;
}
