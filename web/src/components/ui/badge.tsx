import type { RunStatus, RunAgentStatus } from "@/lib/api/client";

type BadgeVariant = "pass" | "fail" | "warn" | "pending" | "neutral";

const variantStyles: Record<BadgeVariant, string> = {
  pass: "text-status-pass bg-status-pass/10",
  fail: "text-status-fail bg-status-fail/10",
  warn: "text-status-warn bg-status-warn/10",
  pending: "text-text-3 bg-surface",
  neutral: "text-text-2 bg-surface",
};

function getRunStatusVariant(status: RunStatus): BadgeVariant {
  switch (status) {
    case "completed": return "pass";
    case "failed": return "fail";
    case "cancelled": return "warn";
    case "running":
    case "scoring":
    case "provisioning":
    case "queued": return "pending";
    case "draft": return "neutral";
  }
}

function getAgentStatusVariant(status: RunAgentStatus): BadgeVariant {
  switch (status) {
    case "completed": return "pass";
    case "failed": return "fail";
    case "executing":
    case "evaluating":
    case "ready":
    case "queued": return "pending";
  }
}

export function Badge({
  variant = "neutral",
  children,
  className = "",
}: {
  variant?: BadgeVariant;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      className={`
        inline-flex items-center
        font-[family-name:var(--font-mono)] text-[11px] font-semibold
        uppercase tracking-[0.06em]
        px-2.5 py-1 rounded
        ${variantStyles[variant]}
        ${className}
      `}
    >
      {children}
    </span>
  );
}

export function RunStatusBadge({ status }: { status: RunStatus }) {
  return <Badge variant={getRunStatusVariant(status)}>{status}</Badge>;
}

export function AgentStatusBadge({ status }: { status: RunAgentStatus }) {
  return <Badge variant={getAgentStatusVariant(status)}>{status}</Badge>;
}
