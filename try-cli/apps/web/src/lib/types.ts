export interface DemoMeta {
  slug: string;
  name: string;
  docs?: string;
  github?: string;
  welcome?: string;
  commands: { label: string; run: string }[];
  sessionMinutes: number;
}
