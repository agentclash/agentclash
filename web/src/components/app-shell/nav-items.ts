import {
  Bot,
  Rocket,
  Sparkles,
  PackageCheck,
  PackageOpen,
  Play,
  FlaskConical,
  ClipboardCheck,
  GitCompare,
  ShieldCheck,
  Workflow,
  Settings2,
  Key,
  Wrench,
  Database,
  FileArchive,
  Lock,
  Cog,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  label: string;
  href: (workspaceId: string) => string;
  icon: LucideIcon;
}

export interface NavSection {
  title: string;
  items: NavItem[];
}

export const navSections: NavSection[] = [
  {
    title: "Agents",
    items: [
      {
        label: "Agent Tryouts",
        href: (id) => `/workspaces/${id}/agent-tryouts`,
        icon: Sparkles,
      },
      {
        label: "Builds",
        href: (id) => `/workspaces/${id}/builds`,
        icon: Bot,
      },
      {
        label: "Deployments",
        href: (id) => `/workspaces/${id}/deployments`,
        icon: Rocket,
      },
      {
        label: "Agent Harnesses",
        href: (id) => `/workspaces/${id}/agent-harnesses`,
        icon: PackageCheck,
      },
    ],
  },
  {
    title: "Benchmarks",
    items: [
      {
        label: "Challenge Packs",
        href: (id) => `/workspaces/${id}/challenge-packs`,
        icon: PackageOpen,
      },
      {
        label: "Datasets",
        href: (id) => `/workspaces/${id}/datasets`,
        icon: Database,
      },
      {
        label: "Runs",
        href: (id) => `/workspaces/${id}/runs`,
        icon: Play,
      },
      {
        label: "Playgrounds",
        href: (id) => `/workspaces/${id}/playgrounds`,
        icon: FlaskConical,
      },
      {
        label: "Regression Suites",
        href: (id) => `/workspaces/${id}/regression-suites`,
        icon: ClipboardCheck,
      },
    ],
  },
  {
    title: "Insights",
    items: [
      {
        label: "Comparisons",
        href: (id) => `/workspaces/${id}/comparisons`,
        icon: GitCompare,
      },
      {
        label: "Release Gates",
        href: (id) => `/workspaces/${id}/release-gates`,
        icon: ShieldCheck,
      },
      {
        label: "CI Setup",
        href: (id) => `/workspaces/${id}/ci-setup`,
        icon: Workflow,
      },
    ],
  },
  {
    title: "Infrastructure",
    items: [
      {
        label: "Runtime Profiles",
        href: (id) => `/workspaces/${id}/runtime-profiles`,
        icon: Settings2,
      },
      {
        label: "Provider Accounts",
        href: (id) => `/workspaces/${id}/provider-accounts`,
        icon: Key,
      },
      {
        label: "Tools",
        href: (id) => `/workspaces/${id}/tools`,
        icon: Wrench,
      },
      {
        label: "Knowledge Sources",
        href: (id) => `/workspaces/${id}/knowledge-sources`,
        icon: Database,
      },
      {
        label: "Artifacts",
        href: (id) => `/workspaces/${id}/artifacts`,
        icon: FileArchive,
      },
      {
        label: "Secrets",
        href: (id) => `/workspaces/${id}/secrets`,
        icon: Lock,
      },
    ],
  },
  {
    title: "Workspace",
    items: [
      {
        label: "Settings",
        href: (id) => `/workspaces/${id}/settings`,
        icon: Cog,
      },
    ],
  },
];
