import { Activity, BookOpen, GitBranch, Search, Terminal } from "lucide-react";
import type { LucideIcon } from "lucide-react";

// Per-card glyph for the use-cases accordion. Indexed by card number
// ("01"–"05") because that's what the Framer-generated component already
// passes through as `SalXds4M0`.
export const CARD_ICONS: Record<string, LucideIcon> = {
  "01": Terminal, // Coding
  "02": BookOpen, // Research
  "03": Activity, // SRE
  "04": GitBranch, // Multi-step ops
  "05": Search, // Codebase Q&A
};
