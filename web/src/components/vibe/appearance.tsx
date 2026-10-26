"use client";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { PortalScope } from "@/components/ui/portal-scope";
import "./workspace.css";
import "./project-workspace.css";
type Appearance = "system" | "light" | "dark";
const AppearanceContext = createContext<{ value: Appearance; setValue: (value: Appearance) => void }>({ value: "system", setValue: () => {} });
export function VibeAppearance({ children }: { children: ReactNode }) {
  const [value, setValue] = useState<Appearance>("system");
  const [systemDark, setSystemDark] = useState(false);
  useEffect(() => {
    try { const saved = localStorage.getItem("vibe-appearance"); if (saved === "light" || saved === "dark" || saved === "system") setValue(saved); } catch { /* Optional preference storage. */ }
    if (typeof matchMedia !== "function") return;
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => setSystemDark(media.matches); update(); media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  const resolved = value === "system" ? systemDark ? "dark" : "light" : value;
  const change = (next: Appearance) => { setValue(next); try { localStorage.setItem("vibe-appearance", next); } catch { /* Remains effective in this page. */ } };
  return <AppearanceContext.Provider value={{ value, setValue: change }}><PortalScope.Provider value={{ className: "vibe-workspace vibe-portal", "data-appearance": resolved }}><main data-appearance={resolved} className="vibe-workspace vibe-route flex h-dvh flex-col overflow-hidden font-sans">{children}</main></PortalScope.Provider></AppearanceContext.Provider>;
}
export function AppearanceControl() {
  const { value, setValue } = useContext(AppearanceContext);
  return <label className="vibe-appearance">Appearance<select aria-label="Appearance" value={value} onChange={event => setValue(event.target.value as Appearance)}><option value="system">System</option><option value="light">Light</option><option value="dark">Dark</option></select></label>;
}
