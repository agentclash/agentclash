import { z } from "zod";

export const CommandSchema = z.object({
  label: z.string(),
  run: z.string(),
});

// Optional "bring your own credentials" block. When present, the UI shows an
// auth panel telling the visitor how to sign in with their own account/key
// inside the sandbox (we never inject provider keys).
export const AuthSchema = z.object({
  provider: z.string().optional(),
  summary: z.string(),
  steps: z.array(z.string()).default([]),
  envKey: z.string().optional(),
  signupUrl: z.string().url().optional(),
});

export const TryCliConfigSchema = z.object({
  name: z.string().min(1),
  slug: z.string().regex(/^[a-z0-9-]+$/).optional(),
  tagline: z.string().optional(),
  category: z.string().optional(),
  image: z.string().default("ubuntu:24.04"),
  template: z.string().optional(),
  docs: z.string().url().optional(),
  github: z.string().url().optional(),
  install: z.array(z.string()).default([]),
  welcome: z.string().optional(),
  commands: z.array(CommandSchema).default([]),
  auth: AuthSchema.optional(),
  sessionMinutes: z.number().int().min(1).max(30).default(10),
});

export type TryCliConfig = z.infer<typeof TryCliConfigSchema>;
export type DemoCommand = z.infer<typeof CommandSchema>;
export type DemoAuth = z.infer<typeof AuthSchema>;

export interface Demo extends TryCliConfig {
  slug: string;
}
