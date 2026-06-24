import { EvalPacksClient } from "./eval-packs-client";

export default async function EvalPacksPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <EvalPacksClient workspaceId={workspaceId} />;
}
