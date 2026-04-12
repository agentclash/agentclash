import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import Link from "next/link";
import { createApiClient } from "@/lib/api/client";
import type { AgentBuildVersion, AgentBuildDetail } from "@/lib/api/types";
import { VersionEditor } from "./version-editor";

export default async function VersionEditorPage({
  params,
}: {
  params: Promise<{
    workspaceId: string;
    buildId: string;
    versionId: string;
  }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId, buildId, versionId } = await params;

  const api = createApiClient(accessToken);
  const [version, build] = await Promise.all([
    api.get<AgentBuildVersion>(`/v1/agent-build-versions/${versionId}`),
    api.get<AgentBuildDetail>(`/v1/agent-builds/${buildId}`),
  ]);

  return (
    <div>
      <div className="flex items-center gap-2 mb-6 text-sm text-muted-foreground">
        <Link
          href={`/workspaces/${workspaceId}/builds`}
          className="hover:text-foreground transition-colors"
        >
          Builds
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <Link
          href={`/workspaces/${workspaceId}/builds/${buildId}`}
          className="hover:text-foreground transition-colors"
        >
          {build.name}
        </Link>
        <span className="text-muted-foreground/40">/</span>
        <span className="text-foreground">v{version.version_number}</span>
      </div>

      <VersionEditor version={version} />
    </div>
  );
}
