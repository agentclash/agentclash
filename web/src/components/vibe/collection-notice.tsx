import type { CollectionStatus } from "@/lib/use-vibe-collection";
import { useVibeBrowserReady } from "@/lib/use-vibe-browser-ready";
import { VibeButton } from "./vibe-button";

export function CollectionNotice({ status, retry, hasRows = false, collection = "saved-work", guest = false }: {
  status?: CollectionStatus;
  hasRows?: boolean;
  retry?: () => void;
  collection?: "projects" | "saved-work";
  guest?: boolean;
}) {
  const ready = useVibeBrowserReady();
  if (!status || status === "ready") return null;
  const projects = collection === "projects";
  const label = projects ? "projects" : "saved work";
  const subject = projects ? "your projects" : "saved work";
  const message = status === "loading" ? `Loading ${label}…`
    : status === "signed_out" ? `Sign in to load ${subject}.`
    : status === "forbidden" && !(projects && guest) ? `You don’t have access to this workspace’s ${label}.`
    : status === "failed" && hasRows ? `Couldn’t refresh ${subject}. Previously loaded ${projects ? "projects are" : "work is"} still shown.`
    : `Couldn’t load ${subject}. Try again.`;
  return <div role="status" className={`vibe-collection-notice ${projects ? "vibe-project-list-notice " : ""}text-sm vibe-muted`}>
    <p>{message}</p>
    {status === "signed_out" && <a href={`/auth/login?returnTo=${encodeURIComponent(!ready ? "/vibe-evals" : window.location.pathname + window.location.search)}`}>Sign in</a>}
    {status !== "loading" && retry && <VibeButton variant="quiet" onClick={retry}>Retry {label}</VibeButton>}
  </div>;
}
