import type { Demo, DemoAuth } from "@try-cli/core";

export interface SessionInfo {
  id: string;
  slug: string;
  expiresAt: number;
  status: "starting" | "ready" | "expired" | "error";
  error?: string;
}

export interface DemoMeta {
  slug: string;
  name: string;
  tagline?: string;
  category?: string;
  docs?: string;
  github?: string;
  welcome?: string;
  commands: { label: string; run: string }[];
  auth?: DemoAuth;
  sessionMinutes: number;
}

export function demoToMeta(demo: Demo): DemoMeta {
  return {
    slug: demo.slug,
    name: demo.name,
    tagline: demo.tagline,
    category: demo.category,
    docs: demo.docs,
    github: demo.github,
    welcome: demo.welcome,
    commands: demo.commands,
    auth: demo.auth,
    sessionMinutes: demo.sessionMinutes,
  };
}
