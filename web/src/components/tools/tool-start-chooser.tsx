"use client";

import Link from "next/link";
import { ArrowRight, Boxes, Globe, MessageSquareDashed, Terminal, Workflow } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";

/**
 * The first screen of tool creation. Instead of asking a non-engineer to choose
 * between "primitive" and "composed", it asks what they want the tool to do and
 * routes to the builder with a sensible starting point preselected.
 */
export function ToolStartChooser({ workspaceId }: { workspaceId: string }) {
  // The chooser is the classic form-editor entry; keep links in form mode.
  const base = `/workspaces/${workspaceId}/tools/new?editor=form`;

  return (
    <div>
      <PageHeader
        breadcrumbs={[
          { label: "Tools", href: `/workspaces/${workspaceId}/tools` },
          { label: "New tool" },
        ]}
        title="What should this tool do?"
        description="Pick a starting point. You can change everything in the next step."
      />

      <div className="grid gap-3 lg:grid-cols-2">
        <ChoiceCard
          href={`${base}&type=primitive`}
          icon={<Boxes className="size-5" />}
          title="A single action"
          description="The tool does one thing — call an API, run a command, read a file, or return a canned response for testing."
          quickStarts={[
            { href: `${base}&type=primitive&start=api`, icon: <Globe className="size-3.5" />, label: "Call a web API" },
            { href: `${base}&type=primitive&start=command`, icon: <Terminal className="size-3.5" />, label: "Run a command" },
            { href: `${base}&type=primitive&start=mock`, icon: <MessageSquareDashed className="size-3.5" />, label: "Return a sample response" },
          ]}
        />
        <ChoiceCard
          href={`${base}&type=composed`}
          icon={<Workflow className="size-5" />}
          title="A sequence of steps"
          description="The tool runs several actions in order, passing each result into the next — for example, fetch data, then save it to a file."
        />
      </div>
    </div>
  );
}

function ChoiceCard({
  href,
  icon,
  title,
  description,
  quickStarts,
}: {
  href: string;
  icon: React.ReactNode;
  title: string;
  description: string;
  quickStarts?: { href: string; icon: React.ReactNode; label: string }[];
}) {
  return (
    <div className="flex flex-col rounded-lg border border-border bg-card p-5 transition-colors hover:border-foreground/20">
      <Link href={href} className="group flex flex-col">
        <div className="mb-3 flex size-10 items-center justify-center rounded-lg bg-muted text-foreground">
          {icon}
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-base font-medium tracking-tight">{title}</span>
          <ArrowRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
        </div>
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      </Link>

      {quickStarts && (
        <div className="mt-4 flex flex-wrap gap-1.5 border-t border-border pt-3">
          {quickStarts.map((q) => (
            <Link
              key={q.href}
              href={q.href}
              className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-foreground/30 hover:text-foreground"
            >
              {q.icon}
              {q.label}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
