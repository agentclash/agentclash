export interface DemoAuth {
  provider?: string;
  summary: string;
  steps: string[];
  envKey?: string;
  signupUrl?: string;
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

export interface TrySession {
  id: string;
  slug: string;
  expiresAt: number;
  status: string;
  mock?: boolean;
  error?: string;
}
