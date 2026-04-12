export default async function WorkspaceDashboard({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-1">Dashboard</h1>
      <p className="text-sm text-muted-foreground mb-6">
        Workspace{" "}
        <code className="font-[family-name:var(--font-mono)] text-xs">
          {workspaceId}
        </code>
      </p>
      <div className="rounded-lg border border-border bg-card p-6 text-sm text-muted-foreground">
        Workspace features are coming in future issues.
      </div>
    </div>
  );
}
