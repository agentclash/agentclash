import { readFileSync, existsSync, readdirSync } from "node:fs";
import { join, dirname } from "node:path";
import { parse as parseYaml } from "yaml";
import { TryCliConfigSchema, type Demo, type TryCliConfig } from "./schema.ts";

export function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function parseConfig(raw: unknown): TryCliConfig {
  return TryCliConfigSchema.parse(raw);
}

export function loadConfigFile(path: string): Demo {
  const content = readFileSync(path, "utf-8");
  const parsed = parseYaml(content);
  const config = parseConfig(parsed);
  return {
    ...config,
    slug: config.slug ?? slugify(config.name),
  };
}

export function findConfigFile(cwd: string): string | null {
  const candidates = [".trycli.yml", ".trycli.yaml", "trycli.yml", "trycli.yaml"];
  for (const file of candidates) {
    const full = join(cwd, file);
    if (existsSync(full)) return full;
  }
  return null;
}

export function loadDemoFromDir(demoDir: string, filename: string): Demo {
  return loadConfigFile(join(demoDir, filename));
}

export function loadAllDemos(demoDir: string): Demo[] {
  if (!existsSync(demoDir)) return [];
  return readdirSync(demoDir)
    .filter((f: string) => f.endsWith(".trycli.yml") || f.endsWith(".trycli.yaml"))
    .map((f: string) => loadDemoFromDir(demoDir, f));
}

export const DEFAULT_CONFIG_TEMPLATE = `# .trycli.yml — Interactive CLI demo config
# Docs: https://try-cli.dev/docs

name: "my-cool-cli"
# slug: "mycool"  # optional, defaults to slugified name

image: "ubuntu:24.04"

# Optional links shown in the demo header
# docs: "https://example.com/docs"
# github: "https://github.com/you/your-cli"

install:
  - "curl -fsSL https://example.com/install.sh | bash"

welcome: |
  my-cool-cli is ready.
  Try:
    mycool --help
    mycool init demo

commands:
  - label: "Show help"
    run: "mycool --help"
  - label: "Create demo project"
    run: "mycool init demo && cd demo"

sessionMinutes: 10
`;
