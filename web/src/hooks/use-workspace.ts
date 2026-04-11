"use client";

import { useParams } from "next/navigation";
import { useSession } from "./use-session";

interface WorkspaceContext {
  workspaceId: string;
  workspaceSlug: string;
  role: string;
}

interface UseWorkspaceReturn {
  workspace: WorkspaceContext | null;
  loading: boolean;
  error: Error | null;
}

/**
 * Derives the current workspace context from the URL `workspaceSlug` param
 * cross-referenced with the session's workspace memberships.
 *
 * Expects the route to contain a `[workspaceSlug]` dynamic segment.
 * If the slug doesn't match any membership, workspace is null.
 */
export function useWorkspace(): UseWorkspaceReturn {
  const params = useParams<{ workspaceSlug?: string }>();
  const { session, loading, error } = useSession();

  const workspaceSlug = params?.workspaceSlug;
  let workspace: WorkspaceContext | null = null;

  if (session && workspaceSlug) {
    // Workspace memberships in the session don't carry slugs — they only have IDs.
    // The full mapping (slug -> id) requires /v1/users/me. For now we match by ID
    // if the param looks like a UUID, otherwise return null and let the consumer
    // fetch the workspace details.
    const membership = session.workspace_memberships.find(
      (m) => m.workspace_id === workspaceSlug,
    );

    if (membership) {
      workspace = {
        workspaceId: membership.workspace_id,
        workspaceSlug,
        role: membership.role,
      };
    }
  }

  return { workspace, loading, error };
}
