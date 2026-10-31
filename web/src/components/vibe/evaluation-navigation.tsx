"use client";

import Link from "next/link";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { projectTitle, sidebarPreference, setSidebarPreference } from "@/lib/vibe-project";
import type { CollectionStatus } from "@/lib/use-vibe-collection";
import { CollectionNotice } from "./collection-notice";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { motion, useReducedMotion } from "framer-motion";
import {
  Archive,
  ChevronRight,
  CircleHelp,
  FolderCog,
  House,
  MessageSquare,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  RefreshCw,
  Settings,
} from "lucide-react";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { ClashMark } from "@/components/marketing/clash-mark";
import { terminal, type Session } from "@/lib/vibe";
import { workingBuildArtifact, buildVersion } from "@/lib/vibe-build-timeline";
import { DeleteProject } from "./delete-project";
import { VibeButton } from "./vibe-button";

export function evaluationIdentity(session: Session) {
  const artifacts = session.document.artifacts;
  const artifact =
    session.document.evaluation?.door === "build"
      ? workingBuildArtifact(session)
      : artifacts.at(-1);
  const title = projectTitle(session);
  const version = artifact ? buildVersion(session, artifact) : 0;
  const scope =
    artifact?.kind === "conversation_evaluation"
      ? "Recorded replies"
      : artifact?.agent_prompt
        ? `${artifact.sample ? "Sample agent" : "Agent"} v${version}`
        : artifact ? `Tests v${version}` : session.document.evaluation?.door === "test" ? "No tests yet" : "No agent yet";
  return { title, scope };
}

type Props = {
  actorScope?: string;
  signInReturnPath?: string;
  archiveMode?: "exclude" | "only" | "all";
  onArchiveMode?: (mode: "exclude" | "only" | "all") => void;
  onMetadata?: (target: Session, patch: { title?: string; archived?: boolean }) => Promise<boolean>;
  /** Signed-in entry only: the workspace for new projects; the picker itself is on the entry screen. */
  workspace?: { name?: string; onChange: () => void };
  accountLabel?: string;
  listStatus?: CollectionStatus;
  onRetryList?: () => void;
  enabled?: boolean;
  contexts: Session[];
  session: Session | null;
  choosing: boolean;
  disabled: boolean;
  onDeleted?: (id: string) => void;
  onNew: () => void;
  onSwitch: (id: string) => void;
  onSettings: () => void;
  onSavedWork: () => void;
  children: (toggle: ReactNode) => ReactNode;
};

// Navigation consumes existing context data. It never creates a session or runs a model.
export function EvaluationNavigation(p: Props) {
  const life = useVibeEvidenceOwner(p.actorScope, p.session?.id);
  const [deleting, setDeleting] = useState<Session>();
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => setCollapsed(sidebarPreference()), []);
  const [search, setSearch] = useState("");
  const searchHint = useId();
  const [renaming, setRenaming] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [metadataBusy, setMetadataBusy] = useState(false);
  const [metadataError, setMetadataError] = useState("");
  const desktopActions = useRef<HTMLDetailsElement>(null);
  const mobileActions = useRef<HTMLDetailsElement>(null);
  async function metadata(patch: { title?: string; archived?: boolean }) {
    if (!p.session || !p.onMetadata) return;
    const current = life.capture();
    setMetadataBusy(true);
    setMetadataError("");
    try {
      if (await p.onMetadata(p.session, patch) && current()) setRenaming(false);
    } catch (e) {
      if (current()) {
        setMetadataError((e as Error).message);
        for (const details of [desktopActions.current, mobileActions.current]) {
          if (details) details.open = true;
        }
      }
    } finally {
      if (current()) setMetadataBusy(false);
    }
  }
  useEffect(() => {
    setRenaming(false);
    setTitleDraft("");
    setMetadataError("");
    setMetadataBusy(false);
  }, [life]);
  const [mobileOpen, setMobileOpen] = useState(false);
  const expandButton = useRef<HTMLButtonElement>(null);
  const collapseButton = useRef<HTMLButtonElement>(null);
  const afterDrawerClose = useRef<(() => void) | null>(null);
  const drawerNavigating = useRef(false);
  const reduced = useReducedMotion();
  if (!p.enabled) return p.children(null);
  const contexts = p.contexts
    .map((item) => (item.id === p.session?.id ? p.session : item))
    .filter(
      (item) =>
        p.archiveMode === "all" ||
        (p.archiveMode === "only" ? !!item.archived_at : !item.archived_at),
    );
  if (
    p.session?.document.evaluation &&
    (p.archiveMode === "all" ||
      (p.archiveMode === "only" ? !!p.session.archived_at : !p.session.archived_at)) &&
    !contexts.some((item) => item.id === p.session!.id)
  )
    contexts.push(p.session);
  const guest = !p.actorScope || p.actorScope === "guest";
  const hasProjects = p.contexts.length > 0 || !!p.session?.document.evaluation;
  const filtered = !!p.archiveMode && p.archiveMode !== "exclude";
  const showListTools = !guest || hasProjects || filtered;
  // A short list needs no search; the filter stays reachable so archived projects can be restored.
  const showSearch = contexts.length > 6 || !!search || filtered;
  const navigate = (action: () => void, mobile: boolean) => {
    if (!mobile) {
      action();
      return;
    }
    // Let the drawer release its focus trap before opening another surface.
    afterDrawerClose.current = action;
    drawerNavigating.current = true;
    setMobileOpen(false);
  };

  const navigation = (mobile: boolean) => (
    <>
      <div className="vibe-sidebar-brand">
        <ClashMark className="size-5 vibe-muted" tone="current" />
        <span>Vibe Evals</span>
        {!mobile && (
          <VibeButton
            ref={collapseButton}
            variant="quiet"
            className="vibe-sidebar-collapse"
            aria-label="Collapse sidebar"
            title="Collapse sidebar"
            onClick={() => {
              setSidebarPreference(true);
              setCollapsed(true);
              requestAnimationFrame(() => expandButton.current?.focus());
            }}
          >
            <PanelLeftClose />
          </VibeButton>
        )}
      </div>
      <div className="vibe-sidebar-create">
        <VibeButton disabled={p.disabled} onClick={() => navigate(p.onNew, mobile)}>
          <Plus />
          New project
        </VibeButton>
      </div>
      <nav aria-label="Your projects" className="vibe-sidebar-list">
        {showListTools && (p.onArchiveMode || p.onRetryList) ? (
          <div className="vibe-sidebar-tools">
            {p.onArchiveMode ? (
              <select
                aria-label="Project archive filter"
                className="vibe-sidebar-filter"
                value={p.archiveMode || "exclude"}
                onChange={(e) => p.onArchiveMode?.(e.target.value as "exclude" | "only" | "all")}
              >
                <option value="exclude">Active projects</option>
                <option value="only">Archived projects</option>
                <option value="all">All projects</option>
              </select>
            ) : <p className="vibe-sidebar-label">Your projects</p>}
            {p.onRetryList && (
              <VibeButton
                variant="quiet"
                className="vibe-sidebar-icon"
                aria-label="Refresh projects"
                title="Refresh projects"
                disabled={p.listStatus === "loading"}
                onClick={p.onRetryList}
              >
                <RefreshCw aria-hidden="true" />
              </VibeButton>
            )}
          </div>
        ) : <p className="vibe-sidebar-label">Your projects</p>}
        {showListTools && showSearch && (
          <>
            <input
              type="search"
              aria-label="Search projects"
              aria-describedby={`${searchHint}-${mobile ? "sheet" : "side"}`}
              placeholder="Search projects"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="vibe-input vibe-sidebar-search"
            />
            <p id={`${searchHint}-${mobile ? "sheet" : "side"}`} className="sr-only">Searches the latest 100 projects loaded here.</p>
          </>
        )}
        <CollectionNotice
          collection="projects"
          guest={guest}
          hasRows={contexts.length > 0}
          status={p.signInReturnPath && p.listStatus === "loading" ? undefined : p.listStatus}
          retry={p.onRetryList}
        />
        {contexts.length
          ? contexts
              .filter((context) =>
                projectTitle(context).toLocaleLowerCase().includes(search.toLocaleLowerCase()),
              )
              .map((context) => {
                const { title, scope } = evaluationIdentity(context);
                const latestRun = context.operations.findLast(
                  (operation) => operation.kind === "check" || operation.kind === "retest",
                );
                const running = context.operations.some((operation) => !terminal(operation.state));
                const selected = context.id === p.session?.id && !p.choosing;
                const status = running ? "Running"
                  : latestRun?.state === "COMPLETED" ? "Run complete"
                  : latestRun?.state === "FAILED" ? "Run failed"
                  : latestRun ? "Run incomplete" : scope;
                return (
                  <button
                    key={context.id}
                    type="button"
                    className="vibe-sidebar-item"
                    data-evaluation-id={context.id}
                    aria-current={selected ? "page" : undefined}
                    disabled={p.disabled}
                    title={`${title} · ${scope}`}
                    onClick={() => navigate(() => p.onSwitch(context.id), mobile)}
                  >
                    <MessageSquare size={16} aria-hidden="true" />
                    <span className="vibe-sidebar-item-copy">
                      <span className="vibe-project-full-title">{title}</span>
                      <small>{status}</small>
                    </span>
                    {selected && (
                      <motion.span
                        className="vibe-sidebar-selected"
                        layoutId={mobile ? "vibe-mobile-selection" : "vibe-desktop-selection"}
                        transition={{ duration: reduced ? 0 : 0.18 }}
                      />
                    )}
                  </button>
                );
              })
          : (!p.listStatus || p.listStatus === "ready") && (
              <p className="vibe-sidebar-empty">No projects yet.</p>
            )}
      </nav>
      <div className="vibe-sidebar-footer">
        <p className="vibe-muted">{p.accountLabel || "Guest · private to this browser"}</p>
        {p.workspace && (
          <div className="vibe-sidebar-workspace">
            <p><span>Workspace</span>{p.workspace.name || "Not chosen"}</p>
            <VibeButton variant="quiet" aria-label={p.workspace.name ? "Change workspace" : "Choose workspace"} onClick={() => navigate(p.workspace!.onChange, mobile)}>
              {p.workspace.name ? "Change" : "Choose"}
            </VibeButton>
          </div>
        )}
        <Link href="/"><House aria-hidden="true" />Home</Link>
        <details>
          <summary><CircleHelp aria-hidden="true" />Help<ChevronRight className="vibe-chevron" aria-hidden="true" /></summary>
          <p>
            Search finds titles in the latest 100 projects. Rename changes the project title.
            Archive hides a project from the active list without stopping runs or deleting
            evidence. To restore it, show Archived projects above the list.
          </p>
          <p>
            Export starts with the selected version and result. Full history is an explicit choice;
            missing evidence needs your confirmation for a partial download. Save creates a
            workspace copy, not a deployment. Use the advanced editor for supported rich packs and
            keep the original download.
          </p>
        </details>
        {p.session && (p.onMetadata || (p.session.document.evaluation && p.onDeleted)) && (
          <details className="vibe-project-actions" key={p.session.id} ref={mobile ? mobileActions : desktopActions}>
            <summary><FolderCog aria-hidden="true" />Project actions<ChevronRight className="vibe-chevron" aria-hidden="true" /></summary>
            <div className="space-y-2">
              {p.onMetadata && (
                <>
                  <p className="vibe-project-full-title">{projectTitle(p.session)}</p>
                  {renaming ? (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        void metadata({ title: titleDraft });
                      }}
                    >
                      <label>
                        Project title
                        <input
                          aria-label="Project title"
                          value={titleDraft}
                          onChange={(e) => setTitleDraft(e.target.value)}
                          className="vibe-input w-full"
                        />
                      </label>
                      <VibeButton type="submit" disabled={metadataBusy || !titleDraft.trim()}>
                        Save title
                      </VibeButton>
                      <VibeButton type="button" variant="quiet" onClick={() => setRenaming(false)}>
                        Cancel rename
                      </VibeButton>
                    </form>
                  ) : (
                    <VibeButton
                      disabled={metadataBusy}
                      onClick={() => {
                        setTitleDraft(projectTitle(p.session!));
                        setRenaming(true);
                      }}
                    >
                      Rename project
                    </VibeButton>
                  )}
                  <VibeButton
                    disabled={metadataBusy}
                    onClick={() => void metadata({ archived: !p.session!.archived_at })}
                  >
                    {p.session.archived_at ? "Restore project" : "Archive project"}
                  </VibeButton>
                  {metadataError && <p role="alert">{metadataError}</p>}
                </>
              )}
              {p.session.document.evaluation && p.onDeleted && (
                <VibeButton
                  variant="quiet"
                  disabled={p.disabled}
                  onClick={() => navigate(() => setDeleting(p.session!), mobile)}
                >
                  Delete this project
                </VibeButton>
              )}
            </div>
          </details>
        )}
        <VibeButton variant="quiet" onClick={() => navigate(p.onSavedWork, mobile)}>
          <Archive />
          Saved work
        </VibeButton>
        <VibeButton variant="quiet" onClick={() => navigate(p.onSettings, mobile)}>
          <Settings />
          Settings
        </VibeButton>
      </div>
    </>
  );
  const toggle = (
    <>
      {collapsed && (
        <VibeButton
          ref={expandButton}
          variant="quiet"
          className="vibe-desktop-toggle"
          aria-label="Expand sidebar"
          title="Expand sidebar"
          onClick={() => {
            setSidebarPreference(false);
            setCollapsed(false);
            requestAnimationFrame(() => collapseButton.current?.focus());
          }}
        >
          <PanelLeftOpen />
        </VibeButton>
      )}
      <Sheet
        open={mobileOpen}
        onOpenChange={(open) => {
          if (open) drawerNavigating.current = false;
          setMobileOpen(open);
        }}
        onOpenChangeComplete={(open) => {
          if (!open) {
            const action = afterDrawerClose.current;
            afterDrawerClose.current = null;
            action?.();
          }
        }}
      >
        <SheetTrigger
          render={
            <VibeButton variant="quiet" className="vibe-mobile-toggle" aria-label="Open projects" />
          }
        >
          <PanelLeftOpen />{guest && !hasProjects && <span>Projects</span>}
        </SheetTrigger>
        <SheetContent
          side="left"
          className="vibe-workspace vibe-sidebar-sheet"
          finalFocus={() => !drawerNavigating.current}
        >
          <SheetTitle className="sr-only">Your projects</SheetTitle>
          <SheetDescription className="sr-only">
            Switch projects or start a new one.
          </SheetDescription>
          {navigation(true)}
        </SheetContent>
      </Sheet>
    </>
  );
  return <>
    {deleting && p.onDeleted && (
      <DeleteProject
        session={deleting}
        onClose={() => setDeleting(undefined)}
        onDeleted={p.onDeleted}
      />
    )}
    {guest && !hasProjects && !filtered && !search ? <div className="vibe-shell vibe-shell-empty"><div className="vibe-main">
    {p.children(null)}
    <CollectionNotice collection="projects" guest status={p.signInReturnPath && p.listStatus === "loading" ? undefined : p.listStatus} retry={p.onRetryList} />
    <header className="vibe-entry-topbar"><div className="vibe-entry-brand"><ClashMark className="size-7" tone="current" /><span>Vibe Evals</span></div><nav aria-label="Entry navigation"><div className="vibe-entry-projects">{toggle}</div><Link href="/docs/vibe-evals">Help</Link><Link href={`/auth/login?returnTo=${encodeURIComponent(p.signInReturnPath || "/vibe-evals")}`}>Sign in</Link></nav></header>
  </div></div> : (
    <div className="vibe-shell" data-collapsed={collapsed}>
      <motion.aside
        className="vibe-sidebar"
        aria-label="Projects sidebar"
        inert={collapsed}
        aria-hidden={collapsed || undefined}
        initial={false}
        animate={{ width: collapsed ? 0 : 252, opacity: collapsed ? 0 : 1 }}
        transition={{ duration: reduced ? 0 : 0.2, ease: [0.22, 1, 0.36, 1] }}
      >
        <div className="vibe-sidebar-inner">{navigation(false)}</div>
      </motion.aside>
      <div className="vibe-main">{p.children(toggle)}</div>
    </div>
  )}</>;
}
