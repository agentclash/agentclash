export interface DemoMeta {
  slug: string;
  name: string;
  docs?: string;
  github?: string;
  welcome?: string;
  commands: { label: string; run: string }[];
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
