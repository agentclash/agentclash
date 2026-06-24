import { LibraryClient } from "./library-client";

export default async function EvalPackLibraryPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <LibraryClient workspaceId={workspaceId} />;
}
