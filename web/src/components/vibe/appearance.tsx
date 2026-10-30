"use client";
import { createContext, useContext, useEffect, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { PortalScope } from "@/components/ui/portal-scope";
import "./workspace.css";
import "./project-workspace.css";
import "./vibe-type.css";
type Appearance = "system" | "light" | "dark";
const AppearanceContext = createContext<{ value: Appearance; setValue: (value: Appearance) => void }>({ value: "system", setValue: () => {} });
const preferenceQuery = "(prefers-color-scheme: dark)";
function readAppearance(): Appearance {
  try { const value = localStorage.getItem("vibe-appearance"); if (value === "light" || value === "dark") return value; } catch { /* Optional preference storage. */ }
  return "system";
}
function subscribeAppearance(notify: () => void) { window.addEventListener("storage", notify); return () => window.removeEventListener("storage", notify); }
function subscribeSystem(notify: () => void) {
  if (typeof matchMedia !== "function") return () => {};
  const media = matchMedia(preferenceQuery); media.addEventListener("change", notify);
  return () => media.removeEventListener("change", notify);
}
export function VibeAppearance({ children }: { children: ReactNode }) {
  const saved = useSyncExternalStore(subscribeAppearance, readAppearance, () => "system" as Appearance);
  const [choice, setValue] = useState<Appearance>();
  const value = choice || saved;
  const systemDark = useSyncExternalStore(subscribeSystem, () => typeof matchMedia === "function" && matchMedia(preferenceQuery).matches, () => false);
  const resolved = value === "system" ? systemDark ? "dark" : "light" : value;
  const route = useRef<HTMLElement>(null);
  // page.tsx sets the browser bar colour per system appearance. A Settings choice can differ from
  // the system, so the bar takes the canvas colour of the resolved appearance while the route is open.
  useEffect(() => {
    const canvas = route.current && getComputedStyle(route.current).getPropertyValue("--vibe-bg").trim();
    if (!canvas) return;
    const metas = [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')];
    const before = metas.map(meta => meta.content);
    metas.forEach(meta => { meta.content = canvas; });
    return () => metas.forEach((meta, i) => { meta.content = before[i]; });
  }, [resolved]);
  const change = (next: Appearance) => { setValue(next); try { localStorage.setItem("vibe-appearance", next); } catch { /* Remains effective in this page. */ } };
  return <AppearanceContext.Provider value={{ value, setValue: change }}><PortalScope.Provider value={{ className: "vibe-workspace vibe-portal", "data-appearance": resolved }}><main ref={route} data-appearance={resolved} className="vibe-workspace vibe-route flex h-dvh flex-col overflow-hidden font-sans">{children}</main></PortalScope.Provider></AppearanceContext.Provider>;
}
export function AppearanceControl() {
  const { value, setValue } = useContext(AppearanceContext);
  return <label className="vibe-appearance">Appearance<select aria-label="Appearance" value={value} onChange={event => setValue(event.target.value as Appearance)}><option value="system">System</option><option value="light">Light</option><option value="dark">Dark</option></select></label>;
}
