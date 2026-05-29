import type { Demo } from "@try-cli/core";

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
  docs?: string;
  github?: string;
  welcome?: string;
  commands: { label: string; run: string }[];
  sessionMinutes: number;
}

export function demoToMeta(demo: Demo): DemoMeta {
  return {
    slug: demo.slug,
    name: demo.name,
    docs: demo.docs,
    github: demo.github,
    welcome: demo.welcome,
    commands: demo.commands,
    sessionMinutes: demo.sessionMinutes,
  };
}
