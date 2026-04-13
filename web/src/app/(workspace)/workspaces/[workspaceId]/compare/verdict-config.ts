import type { ReleaseGateVerdict } from "@/lib/api/types";
import {
  ShieldCheck,
  ShieldAlert,
  ShieldX,
  ShieldQuestion,
  type LucideIcon,
} from "lucide-react";

export interface VerdictStyle {
  variant: "default" | "secondary" | "destructive" | "outline";
  icon: LucideIcon;
  label: string;
  border: string;
  bg: string;
}

export const VERDICT_CONFIG: Record<ReleaseGateVerdict, VerdictStyle> = {
  pass: {
    variant: "default",
    icon: ShieldCheck,
    label: "Pass",
    border: "border-emerald-500/30",
    bg: "bg-emerald-500/5",
  },
  warn: {
    variant: "secondary",
    icon: ShieldAlert,
    label: "Warn",
    border: "border-amber-500/30",
    bg: "bg-amber-500/5",
  },
  fail: {
    variant: "destructive",
    icon: ShieldX,
    label: "Fail",
    border: "border-red-500/30",
    bg: "bg-red-500/5",
  },
  insufficient_evidence: {
    variant: "outline",
    icon: ShieldQuestion,
    label: "Insufficient Evidence",
    border: "border-border",
    bg: "bg-muted/30",
  },
};

export function outcomeColor(outcome: string): string {
  switch (outcome) {
    case "pass":
      return "text-emerald-400";
    case "warn":
      return "text-amber-400";
    case "fail":
      return "text-red-400";
    default:
      return "text-muted-foreground";
  }
}
