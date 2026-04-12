import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { SignOutButton } from "@/app/dashboard/sign-out-button";

export default async function WorkspacePage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { user } = await withAuth();
  if (!user) redirect("/auth/login");

  const { workspaceId } = await params;

  return (
    <div className="min-h-screen">
      <header className="flex h-14 items-center justify-between border-b border-border px-6">
        <a
          href="/dashboard"
          className="font-[family-name:var(--font-display)] text-lg text-foreground/90"
        >
          AgentClash
        </a>
        <div className="flex items-center gap-4">
          <span className="text-sm text-muted-foreground">
            {user.firstName || user.email}
          </span>
          <SignOutButton />
        </div>
      </header>

      <main className="mx-auto max-w-4xl px-6 py-8">
        <h1 className="text-lg font-semibold tracking-tight mb-1">
          Workspace
        </h1>
        <p className="text-sm text-muted-foreground mb-6">
          Workspace ID: <code className="font-[family-name:var(--font-mono)] text-xs">{workspaceId}</code>
        </p>
        <div className="rounded-lg border border-border bg-card p-6 text-sm text-muted-foreground">
          This is a placeholder — workspace features are coming in future issues.
        </div>
      </main>
    </div>
  );
}
