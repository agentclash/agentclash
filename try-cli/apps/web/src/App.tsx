import { useEffect, useState } from "react";
import LandingPage from "./pages/LandingPage";
import DemoPage from "./pages/DemoPage";

function getSlug(): string | null {
  const path = window.location.pathname.replace(/^\//, "");
  if (!path || path === "index.html") return null;
  if (path.startsWith("api") || path.startsWith("badge") || path.startsWith("ws")) return null;
  return path.split("/")[0] ?? null;
}

export default function App() {
  const [slug, setSlug] = useState(getSlug);

  useEffect(() => {
    const onPop = () => setSlug(getSlug());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  if (slug) return <DemoPage slug={slug} />;
  return <LandingPage onNavigate={(s) => { history.pushState({}, "", `/${s}`); setSlug(s); }} />;
}
