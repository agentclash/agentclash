import fs from "fs";
import path from "path";
import matter from "gray-matter";

const CONTENT_DIR = path.join(process.cwd(), "content", "docs");
const REPO_ROOT = path.join(process.cwd(), "..");
const CLI_CMD_DIR = path.join(REPO_ROOT, "cli", "cmd");
const API_CONFIG_FILE = path.join(
  REPO_ROOT,
  "backend",
  "internal",
  "api",
  "config.go",
);
const WORKER_CONFIG_FILE = path.join(
  REPO_ROOT,
  "backend",
  "internal",
  "worker",
  "config.go",
);
const CLI_CONFIG_FILE = path.join(
  REPO_ROOT,
  "cli",
  "internal",
  "config",
  "manager.go",
);
const BACKEND_ENV_FILE = path.join(REPO_ROOT, "backend", ".env.example");

export const DOCS_ORIGIN = "https://agentclash.dev";

export type DocNavItem = {
  title: string;
  description: string;
  slug: string[];
  href: string;
};

export type DocNavSection = {
  title: string;
  description: string;
  items: DocNavItem[];
};

export type DocHeading = {
  id: string;
  text: string;
  level: number;
};

export type DocSearchItem = {
  title: string;
  description: string;
  href: string;
  searchText: string;
};

export type DocPage = {
  slug: string[];
  href: string;
  title: string;
  description: string;
  content: string;
  sectionTitle?: string;
  headings: DocHeading[];
};

type GeneratedDocDefinition = {
  title: string;
  description: string;
  sectionTitle: string;
  buildContent: () => string;
};

type ParsedFlag = {
  name: string;
  shorthand?: string;
  description: string;
  defaultValue?: string;
  required?: boolean;
  persistent?: boolean;
};

type ParsedCommand = {
  id: string;
  use: string;
  short: string;
  flags: ParsedFlag[];
  children: string[];
};

type EnvRow = {
  name: string;
  defaultValue?: string;
  source: string;
  description: string;
};

export const DOCS_NAV: DocNavSection[] = [
  {
    title: "Getting Started",
    description: "Get from first login to a real run, locally or against staging.",
    items: [
      {
        title: "Quickstart",
        description:
          "Use the hosted backend and validate auth, workspace access, and run creation.",
        slug: ["getting-started", "quickstart"],
        href: "/docs/getting-started/quickstart",
      },
      {
        title: "Self-Host",
        description:
          "Bring up the local stack with Postgres, Temporal, API server, worker, and web app.",
        slug: ["getting-started", "self-host"],
        href: "/docs/getting-started/self-host",
      },
      {
        title: "First Eval",
        description:
          "Walk through the current happy path from seeded data to live run events and ranking output.",
        slug: ["getting-started", "first-eval"],
        href: "/docs/getting-started/first-eval",
      },
    ],
  },
  {
    title: "Concepts",
    description:
      "The mental models that matter before you start comparing agents.",
    items: [
      {
        title: "Runs and Evals",
        description:
          "Understand the difference between a run, a ranked result set, and the broader eval concept.",
        slug: ["concepts", "runs-and-evals"],
        href: "/docs/concepts/runs-and-evals",
      },
      {
        title: "Agents and Deployments",
        description:
          "See how runnable agent targets are modeled before they can participate in an eval.",
        slug: ["concepts", "agents-and-deployments"],
        href: "/docs/concepts/agents-and-deployments",
      },
      {
        title: "Challenge Packs and Inputs",
        description:
          "Understand how tasks, input sets, and scoring context are grouped into repeatable workloads.",
        slug: ["concepts", "challenge-packs-and-inputs"],
        href: "/docs/concepts/challenge-packs-and-inputs",
      },
      {
        title: "Replay and Scorecards",
        description:
          "Learn how canonical events become timelines, evidence, and comparison-ready outputs.",
        slug: ["concepts", "replay-and-scorecards"],
        href: "/docs/concepts/replay-and-scorecards",
      },
    ],
  },
  {
    title: "Guides",
    description:
      "Task-oriented walkthroughs for reading results and using the docs with AI tools.",
    items: [
      {
        title: "Interpret Results",
        description:
          "Read timelines, scorecards, and ranking changes without getting lost in raw event volume.",
        slug: ["guides", "interpret-results"],
        href: "/docs/guides/interpret-results",
      },
      {
        title: "Use with AI Tools",
        description:
          "Use llms.txt, the full bundle, and per-page markdown exports with assistants and coding agents.",
        slug: ["guides", "use-with-ai-tools"],
        href: "/docs/guides/use-with-ai-tools",
      },
    ],
  },
  {
    title: "Reference",
    description:
      "Reference surfaces generated from current source readers where possible.",
    items: [
      {
        title: "CLI",
        description:
          "Commands, flags, and command groups generated from the Cobra source tree.",
        slug: ["reference", "cli"],
        href: "/docs/reference/cli",
      },
      {
        title: "Config",
        description:
          "Current environment surface pulled from the API, worker, CLI, and example config sources.",
        slug: ["reference", "config"],
        href: "/docs/reference/config",
      },
    ],
  },
  {
    title: "Architecture",
    description:
      "System boundaries, runtime components, and why the stack is shaped this way.",
    items: [
      {
        title: "Overview",
        description:
          "Web, API, worker, Postgres, Temporal, sandbox, and artifact storage in one picture.",
        slug: ["architecture", "overview"],
        href: "/docs/architecture/overview",
      },
      {
        title: "Orchestration",
        description:
          "How API requests become Temporal workflows and how the worker executes them.",
        slug: ["architecture", "orchestration"],
        href: "/docs/architecture/orchestration",
      },
      {
        title: "Sandbox Layer",
        description:
          "Why execution is isolated behind a provider boundary and how E2B fits today.",
        slug: ["architecture", "sandbox-layer"],
        href: "/docs/architecture/sandbox-layer",
      },
      {
        title: "Data Model",
        description:
          "The core entities behind workspaces, deployments, challenge packs, runs, and evidence.",
        slug: ["architecture", "data-model"],
        href: "/docs/architecture/data-model",
      },
      {
        title: "Evidence Loop",
        description:
          "How run events, artifacts, and scorecards move from execution into replay and review.",
        slug: ["architecture", "evidence-loop"],
        href: "/docs/architecture/evidence-loop",
      },
      {
        title: "Frontend",
        description:
          "How the Next.js app is split between public product pages, authenticated app routes, and docs.",
        slug: ["architecture", "frontend"],
        href: "/docs/architecture/frontend",
      },
    ],
  },
  {
    title: "Contributing",
    description: "Get the repo running and understand where to start making changes.",
    items: [
      {
        title: "Setup",
        description:
          "Clone the repo, boot the local stack, and choose the fastest dev loop for your task.",
        slug: ["contributing", "setup"],
        href: "/docs/contributing/setup",
      },
      {
        title: "Codebase Tour",
        description:
          "Map the top-level modules before you start changing APIs, workflows, or the web app.",
        slug: ["contributing", "codebase-tour"],
        href: "/docs/contributing/codebase-tour",
      },
      {
        title: "Testing",
        description:
          "Pick the smallest useful validation loop and use review checkpoints for scoped changes.",
        slug: ["contributing", "testing"],
        href: "/docs/contributing/testing",
      },
    ],
  },
];

const GENERATED_DOCS: Record<string, GeneratedDocDefinition> = {
  "reference/cli": {
    title: "CLI Reference",
    description:
      "Commands, flags, and command groups generated from the current Cobra CLI source.",
    sectionTitle: "Reference",
    buildContent: renderCLIReference,
  },
  "reference/config": {
    title: "Config Reference",
    description:
      "Environment variables and config precedence generated from the current source readers.",
    sectionTitle: "Reference",
    buildContent: renderConfigReference,
  },
};

const CONFIG_DESCRIPTIONS: Record<string, string> = {
  AGENTCLASH_API_URL: "Override the CLI API base URL.",
  AGENTCLASH_DEV_ORG_MEMBERSHIPS:
    "Inject development org memberships into the CLI dev-auth path.",
  AGENTCLASH_DEV_USER_ID: "Inject a development user ID for CLI dev mode.",
  AGENTCLASH_DEV_WORKSPACE_MEMBERSHIPS:
    "Inject development workspace memberships into the CLI dev-auth path.",
  AGENTCLASH_ORG: "Override the default organization ID for CLI commands.",
  AGENTCLASH_TOKEN: "Provide a CLI token directly, mainly for CI or automation.",
  AGENTCLASH_WORKSPACE:
    "Override the default workspace ID for CLI commands.",
  API_SERVER_BIND_ADDRESS: "Bind address for the API server process.",
  APP_ENV: "Select development, staging, or production behavior.",
  ARTIFACT_MAX_UPLOAD_BYTES:
    "Upper bound for artifact upload size accepted by the API server.",
  ARTIFACT_SIGNED_URL_TTL_SECONDS:
    "Expiry window for signed artifact URLs returned by the API server.",
  ARTIFACT_SIGNING_SECRET:
    "Signing secret for artifact URL generation; required outside local filesystem dev mode.",
  ARTIFACT_STORAGE_BACKEND:
    "Choose filesystem or S3-compatible artifact storage.",
  ARTIFACT_STORAGE_BUCKET: "Artifact bucket or logical container name.",
  ARTIFACT_STORAGE_FILESYSTEM_ROOT:
    "Local artifact root when the filesystem backend is in use.",
  ARTIFACT_STORAGE_S3_ACCESS_KEY_ID:
    "Access key for S3-compatible artifact storage.",
  ARTIFACT_STORAGE_S3_ENDPOINT:
    "Optional custom endpoint for S3-compatible artifact storage.",
  ARTIFACT_STORAGE_S3_FORCE_PATH_STYLE:
    "Toggle path-style addressing for S3-compatible storage.",
  ARTIFACT_STORAGE_S3_REGION: "Region for S3-compatible artifact storage.",
  ARTIFACT_STORAGE_S3_SECRET_ACCESS_KEY:
    "Secret key for S3-compatible artifact storage.",
  AUTH_MODE: "Select dev headers or WorkOS-backed authentication for the API.",
  CORS_ALLOWED_ORIGINS:
    "Allowed browser origins for the API in WorkOS mode.",
  DATABASE_URL: "Postgres connection string.",
  E2B_API_BASE_URL: "Optional E2B API base URL override.",
  E2B_API_KEY: "API key for the E2B sandbox provider.",
  E2B_REQUEST_TIMEOUT: "Timeout for E2B sandbox API calls.",
  E2B_TEMPLATE_ID: "Template ID for the E2B sandbox provider.",
  FRONTEND_URL: "Public web origin used in emails and CLI auth links.",
  HOSTED_RUN_CALLBACK_BASE_URL:
    "Base URL the worker uses when calling hosted-run callback endpoints.",
  HOSTED_RUN_CALLBACK_SECRET:
    "Shared secret for hosted-run callback authentication.",
  REDIS_URL: "Enable Redis-backed event fanout and related features.",
  RESEND_API_KEY: "Enable invite email sending through Resend.",
  RESEND_FROM_EMAIL: "Sender address for invite emails.",
  SANDBOX_PROVIDER:
    "Choose unconfigured or e2b for native sandbox execution.",
  TEMPORAL_HOST_PORT: "Temporal frontend address.",
  TEMPORAL_NAMESPACE: "Temporal namespace used by the API and worker.",
  WORKER_IDENTITY: "Logical worker identity label.",
  WORKER_SHUTDOWN_TIMEOUT:
    "Graceful shutdown timeout for the worker process.",
  WORKOS_CLIENT_ID: "WorkOS client ID used when the API is in workos auth mode.",
  WORKOS_ISSUER:
    "Optional WorkOS issuer override for JWT validation.",
};

function sortEntries(a: fs.Dirent, b: fs.Dirent) {
  if (a.isDirectory() && !b.isDirectory()) return -1;
  if (!a.isDirectory() && b.isDirectory()) return 1;
  return a.name.localeCompare(b.name);
}

function slugToHref(slug: string[]) {
  return slug.length === 0 ? "/docs" : `/docs/${slug.join("/")}`;
}

function slugKey(slug: string[]) {
  return slug.join("/");
}

function docPathForSlug(slug: string[]) {
  if (slug.length === 0) {
    return path.join(CONTENT_DIR, "index.mdx");
  }

  return path.join(CONTENT_DIR, ...slug) + ".mdx";
}

function readSlugs(dir: string, prefix: string[] = []): string[][] {
  if (!fs.existsSync(dir)) return [];

  return fs
    .readdirSync(dir, { withFileTypes: true })
    .sort(sortEntries)
    .flatMap((entry) => {
      if (entry.isDirectory()) {
        return readSlugs(path.join(dir, entry.name), [...prefix, entry.name]);
      }

      if (!entry.isFile() || !entry.name.endsWith(".mdx")) {
        return [];
      }

      const stem = entry.name.replace(/\.mdx$/, "");
      if (stem === "index") {
        return [prefix];
      }

      return [[...prefix, stem]];
    });
}

function normalizeWhitespace(value: string) {
  return value.replace(/\s+/g, " ").trim();
}

function stripInlineMarkdown(value: string) {
  return normalizeWhitespace(
    value
      .replace(/`([^`]+)`/g, "$1")
      .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
      .replace(/[*_~>#]/g, "")
      .replace(/<[^>]+>/g, ""),
  );
}

export function slugify(value: string) {
  return stripInlineMarkdown(value)
    .toLowerCase()
    .replace(/&/g, "and")
    .replace(/[^a-z0-9\s-]/g, "")
    .trim()
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

export function extractHeadings(content: string): DocHeading[] {
  return content
    .split(/\r?\n/)
    .flatMap((line) => {
      const match = /^(##|###)\s+(.+)$/.exec(line.trim());
      if (!match) return [];

      const text = stripInlineMarkdown(match[2]);
      if (!text) return [];

      return [
        {
          id: slugify(text),
          text,
          level: match[1].length,
        },
      ];
    });
}

function findSectionTitle(href: string) {
  for (const section of DOCS_NAV) {
    if (section.items.some((item) => item.href === href)) {
      return section.title;
    }
  }

  return undefined;
}

function createDocPage(
  slug: string[],
  title: string,
  description: string,
  content: string,
  sectionTitle?: string,
): DocPage {
  return {
    slug,
    href: slugToHref(slug),
    title,
    description,
    content,
    sectionTitle,
    headings: extractHeadings(content),
  };
}

function getFileDocBySlug(slug: string[]) {
  const filePath = docPathForSlug(slug);
  if (!fs.existsSync(filePath)) return null;

  const raw = fs.readFileSync(filePath, "utf-8");
  const { data, content } = matter(raw);
  const href = slugToHref(slug);

  return createDocPage(
    slug,
    data.title as string,
    data.description as string,
    content,
    findSectionTitle(href),
  );
}

function getGeneratedDocBySlug(slug: string[]) {
  const key = slugKey(slug);
  const generated = GENERATED_DOCS[key];
  if (!generated) return null;

  return createDocPage(
    slug,
    generated.title,
    generated.description,
    generated.buildContent(),
    generated.sectionTitle,
  );
}

function parseGoField(block: string, field: string) {
  const match = block.match(
    new RegExp(`${field}:\\s*(?:"([^"]*)"|\`([\\s\\S]*?)\`)`),
  );

  return normalizeWhitespace(match?.[1] ?? match?.[2] ?? "");
}

function extractCommandBlocks(source: string) {
  const blocks: Array<{ id: string; block: string }> = [];
  const pattern = /var\s+(\w+)\s*=\s*&cobra\.Command\s*{/g;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(source))) {
    const id = match[1];
    const braceStart = pattern.lastIndex - 1;
    let depth = 0;
    let end = braceStart;

    for (let i = braceStart; i < source.length; i += 1) {
      const char = source[i];
      if (char === "{") {
        depth += 1;
      } else if (char === "}") {
        depth -= 1;
        if (depth === 0) {
          end = i;
          break;
        }
      }
    }

    blocks.push({ id, block: source.slice(braceStart + 1, end) });
  }

  return blocks;
}

function parseCobraCommands() {
  const commands = new Map<string, ParsedCommand>();
  const files = fs
    .readdirSync(CLI_CMD_DIR)
    .filter((entry) => entry.endsWith(".go"))
    .map((entry) => fs.readFileSync(path.join(CLI_CMD_DIR, entry), "utf-8"));

  for (const source of files) {
    for (const { id, block } of extractCommandBlocks(source)) {
      const use = parseGoField(block, "Use");
      if (!use) continue;

      commands.set(id, {
        id,
        use,
        short: parseGoField(block, "Short"),
        flags: [],
        children: [],
      });
    }
  }

  for (const source of files) {
    const lines = source.split(/\r?\n/);
    for (const line of lines) {
      const addMatch = line.match(/(\w+)\.AddCommand\(([^)]+)\)/);
      if (addMatch) {
        const parent = commands.get(addMatch[1]);
        if (parent) {
          for (const childID of addMatch[2]
            .split(",")
            .map((value) => value.trim())
            .filter(Boolean)) {
            if (commands.has(childID) && !parent.children.includes(childID)) {
              parent.children.push(childID);
            }
          }
        }
      }

      const flagMatch = line.match(/(\w+)\.(Persistent)?Flags\(\)\.(\w+)\((.*)\)/);
      if (flagMatch) {
        const command = commands.get(flagMatch[1]);
        if (!command) continue;

        const method = flagMatch[3];
        const stringLiterals = [...flagMatch[4].matchAll(/"([^"]*)"/g)].map(
          (value) => value[1],
        );
        if (stringLiterals.length < 2) continue;

        const flag: ParsedFlag = {
          name: stringLiterals[0],
          description: stringLiterals[stringLiterals.length - 1],
          persistent: Boolean(flagMatch[2]),
        };

        if (method.endsWith("P") && stringLiterals[1]) {
          flag.shorthand = stringLiterals[1];
        }

        if (method.includes("Var")) {
          if (stringLiterals.length >= 4) {
            flag.defaultValue = stringLiterals[stringLiterals.length - 2];
          }
        } else if (stringLiterals.length >= 3) {
          flag.defaultValue = stringLiterals[stringLiterals.length - 2];
        }

        command.flags.push(flag);
      }

      const requiredMatch = line.match(/(\w+)\.MarkFlagRequired\("([^"]+)"\)/);
      if (requiredMatch) {
        const command = commands.get(requiredMatch[1]);
        if (!command) continue;

        const flag = command.flags.find((item) => item.name === requiredMatch[2]);
        if (flag) {
          flag.required = true;
        }
      }
    }
  }

  for (const command of commands.values()) {
    command.children.sort((a, b) => {
      const left = commands.get(a)?.use ?? a;
      const right = commands.get(b)?.use ?? b;
      return left.localeCompare(right);
    });
    command.flags.sort((a, b) => a.name.localeCompare(b.name));
  }

  return commands;
}

function formatFlag(flag: ParsedFlag) {
  const pieces = [`\`--${flag.name}\``];
  if (flag.shorthand) {
    pieces.push(`(\`-${flag.shorthand}\`)`);
  }
  if (flag.required) {
    pieces.push("(required)");
  }
  if (flag.defaultValue) {
    pieces.push(`default: \`${flag.defaultValue}\``);
  }

  return `${pieces.join(" ")} — ${flag.description}`;
}

function renderCommandSection(
  commandID: string,
  commands: Map<string, ParsedCommand>,
  depth: number,
  seen = new Set<string>(),
): string[] {
  const command = commands.get(commandID);
  if (!command || seen.has(commandID)) return [];
  seen.add(commandID);

  const heading = "#".repeat(Math.min(depth, 6));
  const lines = [`${heading} \`${command.use}\``, ""];

  if (command.short) {
    lines.push(command.short, "");
  }

  if (command.flags.length > 0) {
    lines.push("Flags", "");
    for (const flag of command.flags) {
      lines.push(`- ${formatFlag(flag)}`);
    }
    lines.push("");
  }

  for (const childID of command.children) {
    lines.push(...renderCommandSection(childID, commands, depth + 1, seen));
  }

  return lines;
}

function renderCLIReference() {
  const commands = parseCobraCommands();
  const root = commands.get("rootCmd");
  const lines = [
    "This page is generated from the Cobra command definitions in `cli/cmd`.",
    "",
    "## Global flags",
    "",
  ];

  if (root?.flags.length) {
    for (const flag of root.flags.filter((item) => item.persistent)) {
      lines.push(`- ${formatFlag(flag)}`);
    }
  } else {
    lines.push("- No persistent flags were discovered.");
  }

  lines.push("", "## Command groups", "");

  for (const childID of root?.children ?? []) {
    lines.push(...renderCommandSection(childID, commands, 3));
  }

  lines.push(
    "## Source pointers",
    "",
    "- `cli/cmd/root.go`",
    "- `cli/cmd/auth.go`",
    "- `cli/cmd/workspace.go`",
    "- `cli/cmd/run.go`",
    "- `cli/cmd/compare.go`",
  );

  return lines.join("\n");
}

function parseConstMap(source: string) {
  const values = new Map<string, string>();
  const constBlocks = [...source.matchAll(/const\s*\(([\s\S]*?)\)/g)].map(
    (match) => match[1],
  );

  for (const block of constBlocks) {
    for (const line of block.split(/\r?\n/)) {
      const trimmed = line.trim().replace(/,$/, "");
      if (!trimmed || trimmed.startsWith("//")) continue;
      const match = trimmed.match(/^(\w+)\s*=\s*(.+)$/);
      if (match) {
        values.set(match[1], normalizeWhitespace(match[2]));
      }
    }
  }

  for (const match of source.matchAll(/^const\s+(\w+)\s*=\s*(.+)$/gm)) {
    values.set(match[1], normalizeWhitespace(match[2].replace(/,$/, "")));
  }

  return values;
}

function resolveFallback(value: string, consts: Map<string, string>) {
  const trimmed = normalizeWhitespace(value);
  return consts.get(trimmed) ?? trimmed;
}

function collectEnvRowsFromGo(filePath: string, sourceLabel: string) {
  const source = fs.readFileSync(filePath, "utf-8");
  const consts = parseConstMap(source);
  const rows = new Map<string, EnvRow>();

  const addRow = (name: string, defaultValue?: string) => {
    rows.set(name, {
      name,
      defaultValue,
      source: sourceLabel,
      description:
        CONFIG_DESCRIPTIONS[name] ?? `Read by ${sourceLabel.toLowerCase()}.`,
    });
  };

  for (const match of source.matchAll(
    /(?:envOrDefault|boolEnvOrDefault|int64EnvOrDefault|durationEnvOrDefault|durationSecondsEnvOrDefault)\("([^"]+)",\s*([^)]+)\)/g,
  )) {
    addRow(match[1], resolveFallback(match[2], consts));
  }

  for (const match of source.matchAll(/(?:os\.(?:Getenv|LookupEnv)|optionalEnv)\("([^"]+)"\)/g)) {
    addRow(match[1]);
  }

  return [...rows.values()].sort((a, b) => a.name.localeCompare(b.name));
}

function collectBackendExampleRows() {
  if (!fs.existsSync(BACKEND_ENV_FILE)) return [];

  const rows = new Map<string, EnvRow>();
  const source = fs.readFileSync(BACKEND_ENV_FILE, "utf-8");

  for (const line of source.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const separator = trimmed.indexOf("=");
    if (separator <= 0) continue;

    const name = trimmed.slice(0, separator);
    const defaultValue = trimmed.slice(separator + 1);
    rows.set(name, {
      name,
      defaultValue,
      source: "backend/.env.example",
      description:
        CONFIG_DESCRIPTIONS[name] ?? "Present in the backend example environment file.",
    });
  }

  return [...rows.values()].sort((a, b) => a.name.localeCompare(b.name));
}

function renderEnvTable(title: string, rows: EnvRow[]) {
  const lines = [`## ${title}`, "", "| Variable | Default | Description |", "| --- | --- | --- |"]; 

  for (const row of rows) {
    lines.push(
      `| \`${row.name}\` | ${row.defaultValue ? `\`${row.defaultValue}\`` : "—"} | ${row.description} |`,
    );
  }

  lines.push("");
  return lines;
}

function renderConfigReference() {
  const apiRows = collectEnvRowsFromGo(API_CONFIG_FILE, "backend/internal/api/config.go");
  const workerRows = collectEnvRowsFromGo(
    WORKER_CONFIG_FILE,
    "backend/internal/worker/config.go",
  );
  const cliRows = collectEnvRowsFromGo(
    CLI_CONFIG_FILE,
    "cli/internal/config/manager.go",
  );
  const backendExampleRows = collectBackendExampleRows();

  const lines = [
    "This page is generated from the config readers in the API server, worker, CLI, and the checked-in backend example environment file.",
    "",
    "## CLI precedence",
    "",
    "- API URL: `--api-url > AGENTCLASH_API_URL > saved user config > http://localhost:8080`",
    "- Workspace: `--workspace > AGENTCLASH_WORKSPACE > project config > user config`",
    "- Output format: `--json > --output > user config > table`",
    "",
    ...renderEnvTable("API Server Environment", apiRows),
    ...renderEnvTable("Worker Environment", workerRows),
    ...renderEnvTable("CLI Environment", cliRows),
    ...renderEnvTable("Backend Example Environment", backendExampleRows),
    "## Source pointers",
    "",
    "- `backend/internal/api/config.go`",
    "- `backend/internal/worker/config.go`",
    "- `cli/internal/config/manager.go`",
    "- `backend/.env.example`",
  ];

  return lines.join("\n");
}

function uniqueSlugs(slugs: string[][]) {
  const seen = new Set<string>();
  return slugs.filter((slug) => {
    const key = slugKey(slug);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function docHrefToMarkdownHref(href: string, origin: string) {
  if (href === "/docs") {
    return `${origin}/docs-md`;
  }

  if (href.startsWith("/docs/")) {
    return `${origin}/docs-md${href.slice("/docs".length)}`;
  }

  return href.startsWith("http") ? href : `${origin}${href}`;
}

function renderCallout(type: string, body: string) {
  const label = type.charAt(0).toUpperCase() + type.slice(1);
  const lines = body
    .trim()
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  if (lines.length === 0) {
    return `> ${label}`;
  }

  return [
    `> ${label}: ${lines[0]}`,
    ...lines.slice(1).map((line) => `> ${line}`),
  ].join("\n");
}

function normalizeMarkdownForExport(content: string, origin: string) {
  return content
    .replace(/<Callout type="(info|warning|note)">([\s\S]*?)<\/Callout>/g, (_, type, body) =>
      renderCallout(type, body),
    )
    .replace(/\]\((\/docs(?:\/[^)\s]*)?)\)/g, (_, href) => `](${docHrefToMarkdownHref(href, origin)})`)
    .trim();
}

export function getDocMarkdownPath(slug: string[] = []) {
  return slug.length === 0 ? "/docs-md" : `/docs-md/${slug.join("/")}`;
}

export function flattenDocsNav() {
  return DOCS_NAV.flatMap((section) => section.items);
}

export function getOrderedDocs() {
  return [
    {
      title: "Overview",
      href: "/docs",
    },
    ...flattenDocsNav().map((item) => ({
      title: item.title,
      href: item.href,
    })),
  ];
}

export function getDocNeighbors(currentHref: string) {
  const ordered = getOrderedDocs();
  const index = ordered.findIndex((item) => item.href === currentHref);
  if (index === -1) return { previous: null, next: null };

  return {
    previous: ordered[index - 1] ?? null,
    next: ordered[index + 1] ?? null,
  };
}

export function getDocBySlug(slug: string[] = []): DocPage | null {
  return getGeneratedDocBySlug(slug) ?? getFileDocBySlug(slug);
}

export function getAllDocSlugs() {
  const generatedSlugs = Object.keys(GENERATED_DOCS).map((value) =>
    value.split("/"),
  );
  return uniqueSlugs([...readSlugs(CONTENT_DIR), ...generatedSlugs]);
}

export function getAllDocPaths() {
  return getAllDocSlugs().map((slug) => slugToHref(slug));
}

export function getAllDocMarkdownPaths() {
  return getAllDocSlugs().map((slug) => getDocMarkdownPath(slug));
}

export function getDocsSearchIndex(): DocSearchItem[] {
  return getAllDocSlugs()
    .map((slug) => getDocBySlug(slug))
    .filter((doc): doc is DocPage => Boolean(doc))
    .map((doc) => ({
      title: doc.title,
      description: doc.description,
      href: doc.href,
      searchText: `${doc.title} ${doc.description} ${doc.headings
        .map((heading) => heading.text)
        .join(" ")} ${stripInlineMarkdown(doc.content).slice(0, 900)}`.toLowerCase(),
    }));
}

export function renderDocMarkdown(doc: DocPage, origin = DOCS_ORIGIN) {
  const lines = [
    `# ${doc.title}`,
    "",
    doc.description,
    "",
    `Source: ${origin}${doc.href}`,
    `Markdown export: ${origin}${getDocMarkdownPath(doc.slug)}`,
    "",
    normalizeMarkdownForExport(doc.content, origin),
  ];

  return lines.join("\n").trim();
}

export function buildLlmsIndex(origin = DOCS_ORIGIN) {
  const lines = [
    "# AgentClash",
    "",
    "> AgentClash runs agents against repeatable challenge packs, captures replay evidence, and shows where a run won, failed, or drifted.",
    "",
    "Use this index when you want the shortest machine-readable map of the public docs. Fetch `/llms-full.txt` for the bundled corpus, or use the `/docs-md/...` links below for page-level markdown exports.",
    "",
    "## Core entrypoints",
    "",
    `- [Docs home](${origin}/docs-md) - overview, navigation, and starting points.`,
    `- [Quickstart](${origin}/docs-md/getting-started/quickstart) - fastest path to a real run.`,
    `- [Self-Host](${origin}/docs-md/getting-started/self-host) - local stack and service dependencies.`,
    `- [First Eval](${origin}/docs-md/getting-started/first-eval) - end-to-end walkthrough of one eval path.`,
    `- [CLI Reference](${origin}/docs-md/reference/cli) - generated command reference.`,
    `- [Config Reference](${origin}/docs-md/reference/config) - generated environment and precedence reference.`,
    `- [Full bundle](${origin}/llms-full.txt) - all shipped docs in one file.`,
    "",
  ];

  for (const section of DOCS_NAV) {
    lines.push(`## ${section.title}`, "");
    for (const item of section.items) {
      lines.push(
        `- [${item.title}](${origin}${getDocMarkdownPath(item.slug)}) - ${item.description}`,
      );
    }
    lines.push("");
  }

  return lines.join("\n").trim();
}

export function buildLlmsFull(origin = DOCS_ORIGIN) {
  const orderedSlugs = uniqueSlugs([[], ...flattenDocsNav().map((item) => item.slug)]);
  const docs = orderedSlugs
    .map((slug) => getDocBySlug(slug))
    .filter((doc): doc is DocPage => Boolean(doc));

  const lines = [
    "# AgentClash Docs Bundle",
    "",
    `Canonical docs home: ${origin}/docs`,
    `Machine-readable index: ${origin}/llms.txt`,
    "",
    "This file concatenates the currently shipped AgentClash docs pages into one markdown-oriented bundle for assistants, coding agents, and local retrieval pipelines.",
  ];

  for (const doc of docs) {
    lines.push("", "---", "", renderDocMarkdown(doc, origin));
  }

  return lines.join("\n").trim();
}
