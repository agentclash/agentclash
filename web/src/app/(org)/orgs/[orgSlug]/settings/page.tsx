import { OrgSettingsGate } from "./org-settings-gate";

export default async function OrgSettingsPage({
  params,
}: {
  params: Promise<{ orgSlug: string }>;
}) {
  const { orgSlug } = await params;

  return <OrgSettingsGate orgSlug={orgSlug} />;
}
