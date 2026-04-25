import { RegressionSuitesClient } from "./regression-suites-client";

export default async function RegressionSuitesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <RegressionSuitesClient workspaceId={workspaceId} />;
}
