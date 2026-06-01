import { CISetupClient } from "./ci-setup-client";

export default async function CISetupPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <CISetupClient workspaceId={workspaceId} />;
}
