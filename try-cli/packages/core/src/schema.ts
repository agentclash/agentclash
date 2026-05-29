import { z } from "zod";

export const CommandSchema = z.object({
  label: z.string(),
  run: z.string(),
});

export const TryCliConfigSchema = z.object({
  name: z.string().min(1),
  slug: z.string().regex(/^[a-z0-9-]+$/).optional(),
  image: z.string().default("ubuntu:24.04"),
  template: z.string().optional(),
  docs: z.string().url().optional(),
  github: z.string().url().optional(),
  install: z.array(z.string()).default([]),
  welcome: z.string().optional(),
  commands: z.array(CommandSchema).default([]),
  sessionMinutes: z.number().int().min(1).max(30).default(10),
});

export type TryCliConfig = z.infer<typeof TryCliConfigSchema>;
export type DemoCommand = z.infer<typeof CommandSchema>;

export interface Demo extends TryCliConfig {
  slug: string;
}
