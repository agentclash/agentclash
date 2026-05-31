import { DatasetsClient } from "./datasets-client";

export default async function DatasetsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <DatasetsClient workspaceId={workspaceId} />;
}
