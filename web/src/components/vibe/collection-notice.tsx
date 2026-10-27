import type { CollectionStatus } from "@/lib/use-vibe-collection";
import { VibeButton } from "./vibe-button";

export function CollectionNotice({ status, retry, hasRows = false, guestProjects = false }: {
  status?: CollectionStatus;
  hasRows?: boolean;
  retry?: () => void;
  guestProjects?: boolean;
}) {
  if (!status || status === "ready") return null;
  if (guestProjects && (status === "forbidden" || status === "failed")) {
    return <div role="status" className="text-sm vibe-muted">
      <p>{status === "failed" && hasRows ? "Couldn’t refresh your projects. Previously loaded projects are still shown." : "Couldn’t load your projects. Try again."}</p>
      {retry && <VibeButton variant="quiet" onClick={retry}>Retry projects</VibeButton>}
    </div>;
  }
  return <div role="status" className="text-sm vibe-muted"><p>{status === "loading" ? "Loading saved work…" : status === "signed_out" ? "Sign in to load saved work." : status === "forbidden" ? "You don’t have access to this workspace’s saved work." : hasRows ? "Couldn’t refresh saved work. Previously loaded work is still shown." : "Couldn’t load saved work. Try again."}</p>{status === "signed_out" && <a href={`/auth/login?returnTo=${encodeURIComponent(typeof window === "undefined" ? "/vibe-evals" : window.location.pathname + window.location.search)}`}>Sign in</a>}{status !== "loading" && <VibeButton variant="quiet" onClick={retry}>Retry saved work</VibeButton>}</div>;
}
