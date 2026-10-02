"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { UserMenu } from "@/components/app-shell/user-menu";
import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { useRef, useState, type ReactNode } from "react";
import { motion, useReducedMotion } from "framer-motion";
import { Archive, MessageSquare, PanelLeftClose, PanelLeftOpen, Plus, Settings } from "lucide-react";
import { Sheet, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { ClashMark } from "@/components/marketing/clash-mark";
import { terminal, type Session } from "@/lib/vibe";
import { evaluationIdentity } from "@/lib/vibe-build-timeline";
import { DeleteProject } from "./delete-project";
import { VibeButton } from "./vibe-button";


type Props = {
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
  const { user, loading } = useAuth();
  const params = useSearchParams();
  const returnTo = sanitizeReturnTo(`/vibe-evals?${params}`);
  const [deleting, setDeleting] = useState<Session>();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const expandButton = useRef<HTMLButtonElement>(null);
  const collapseButton = useRef<HTMLButtonElement>(null);
  const afterDrawerClose = useRef<(() => void) | null>(null);
  const drawerNavigating = useRef(false);
  const reduced = useReducedMotion();
  if (!p.enabled) return p.children(null);
  const contexts = p.contexts.map(item => item.id === p.session?.id ? p.session : item);
  if (p.session?.document.evaluation && !contexts.some(item => item.id === p.session!.id)) contexts.push(p.session);
  const navigate = (action: () => void, mobile: boolean) => {
    if (!mobile) { action(); return; }
    // Let the drawer release its focus trap before opening another surface.
    afterDrawerClose.current = action;
    drawerNavigating.current = true;
    setMobileOpen(false);
  };

  const navigation = (mobile: boolean) => <>
    <div className="vibe-sidebar-brand">
      <ClashMark className="size-5 vibe-muted" /><span>Vibe Evals</span>
      {!mobile && <VibeButton ref={collapseButton} variant="quiet" className="vibe-sidebar-collapse" aria-label="Collapse sidebar" title="Collapse sidebar" onClick={() => {
        setCollapsed(true); requestAnimationFrame(() => expandButton.current?.focus());
      }}><PanelLeftClose /></VibeButton>}
    </div>
    <div className="vibe-sidebar-create">
      <VibeButton disabled={p.disabled} onClick={() => navigate(p.onNew, mobile)}><Plus />New agent</VibeButton>
    </div>
    <nav aria-label="Your agents" className="vibe-sidebar-list">
      <p className="vibe-sidebar-label">Your agents</p>
      {contexts.length ? contexts.map(context => {
        const { title, scope } = evaluationIdentity(context);
        const running = context.operations.some(operation => !terminal(operation.state));
        const selected = context.id === p.session?.id && !p.choosing;
        return <button key={context.id} type="button" className="vibe-sidebar-item" data-evaluation-id={context.id}
          aria-current={selected ? "page" : undefined} disabled={p.disabled}
          title={`${title} · ${scope}`} onClick={() => navigate(() => p.onSwitch(context.id), mobile)}>
          <MessageSquare size={16} aria-hidden="true" />
          <span className="vibe-sidebar-item-copy"><span>{title}</span><small>{scope}{running && " · Running"}</small></span>
          {selected && <motion.span className="vibe-sidebar-selected" layoutId={mobile ? "vibe-mobile-selection" : "vibe-desktop-selection"}
            transition={{ duration: reduced ? 0 : 0.18 }} />}
        </button>;
      }) : <p className="vibe-sidebar-empty">Your work will appear here.</p>}
    </nav>
    <div className="vibe-sidebar-footer">
      <Link href="/">Home</Link>
      {user ? <><Link href="/dashboard">Dashboard</Link><UserMenu displayName={[user.firstName, user.lastName].filter(Boolean).join(" ")} email={user.email} avatarUrl={user.profilePictureUrl || undefined} /></> : !loading && <Link href={`/auth/login?${new URLSearchParams({ returnTo, mode: "signin" })}`}>Sign in</Link>}
      {p.session?.document.evaluation && p.onDeleted && <VibeButton variant="quiet" disabled={p.disabled} onClick={() => navigate(() => setDeleting(p.session!), mobile)}>Delete this project</VibeButton>}
      <VibeButton variant="quiet" onClick={() => navigate(p.onSavedWork, mobile)}><Archive />Saved work</VibeButton>
      <VibeButton variant="quiet" onClick={() => navigate(p.onSettings, mobile)}><Settings />Settings</VibeButton>
    </div>
  </>;
  const toggle = <>
    {collapsed && <VibeButton ref={expandButton} variant="quiet" className="vibe-desktop-toggle" aria-label="Expand sidebar" title="Expand sidebar" onClick={() => {
      setCollapsed(false); requestAnimationFrame(() => collapseButton.current?.focus());
    }}><PanelLeftOpen /></VibeButton>}
    <Sheet open={mobileOpen} onOpenChange={open => {
      if (open) drawerNavigating.current = false;
      setMobileOpen(open);
    }} onOpenChangeComplete={open => {
      if (!open) { const action = afterDrawerClose.current; afterDrawerClose.current = null; action?.(); }
    }}>
      <SheetTrigger render={<VibeButton variant="quiet" className="vibe-mobile-toggle" aria-label="Open agents" />}><PanelLeftOpen /></SheetTrigger>
      <SheetContent side="left" className="vibe-workspace vibe-sidebar-sheet" finalFocus={() => !drawerNavigating.current}>
        <SheetTitle className="sr-only">Your agents</SheetTitle>
        <SheetDescription className="sr-only">Switch agents or start something new.</SheetDescription>
        {navigation(true)}
      </SheetContent>
    </Sheet>
  </>;
  return <div className="vibe-shell" data-collapsed={collapsed}>
    <motion.aside className="vibe-sidebar" aria-label="Agent sidebar" inert={collapsed} aria-hidden={collapsed || undefined}
      initial={false} animate={{ width: collapsed ? 0 : 252, opacity: collapsed ? 0 : 1 }}
      transition={{ duration: reduced ? 0 : 0.2, ease: [0.22, 1, 0.36, 1] }}>
      <div className="vibe-sidebar-inner">{navigation(false)}</div>
    </motion.aside>
    {deleting && p.onDeleted && <DeleteProject session={deleting} onClose={() => setDeleting(undefined)} onDeleted={p.onDeleted} />}
    <div className="vibe-main">{p.children(toggle)}</div>
  </div>;
}
