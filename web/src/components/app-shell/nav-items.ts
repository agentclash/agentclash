import {
  Bot,
  Rocket,
  PackageOpen,
  Play,
  GitCompare,
  ShieldCheck,
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
        label: "Builds",
        href: (id) => `/workspaces/${id}/builds`,
        icon: Bot,
      },
      {
        label: "Deployments",
        href: (id) => `/workspaces/${id}/deployments`,
        icon: Rocket,
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
        label: "Runs",
        href: (id) => `/workspaces/${id}/runs`,
        icon: Play,
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
    ],
  },
];
