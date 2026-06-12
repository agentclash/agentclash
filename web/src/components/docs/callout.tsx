import type { ReactNode } from "react";
import { AlertTriangle, Info, NotebookPen } from "lucide-react";

type CalloutType = "info" | "warning" | "note";

const styles: Record<
  CalloutType,
  {
    icon: typeof Info;
    className: string;
    label: string;
  }
> = {
  info: {
    icon: Info,
    label: "Info",
    className:
      "border-white/[0.12] bg-white/[0.04] text-white/72 [&_strong]:text-white/90",
  },
  warning: {
    icon: AlertTriangle,
    label: "Warning",
    className:
      "border-amber-300/20 bg-amber-300/[0.06] text-amber-50/85 [&_strong]:text-amber-50",
  },
  note: {
    icon: NotebookPen,
    label: "Note",
    className:
      "border-white/[0.1] bg-white/[0.03] text-white/65 [&_strong]:text-white/88",
  },
};

export function Callout({
  type = "info",
  children,
}: {
  type?: CalloutType;
  children: ReactNode;
}) {
  const style = styles[type];
  const Icon = style.icon;

  return (
    <div
      className={`not-prose my-6 rounded-2xl border px-4 py-4 sm:px-5 ${style.className}`}
    >
      <div className="flex items-start gap-3">
        <Icon className="mt-0.5 size-4 shrink-0 opacity-70" />
        <div className="min-w-0 text-sm leading-7">
          <p className="mb-1 text-2xs font-semibold uppercase tracking-[0.14em] opacity-80">
            {style.label}
          </p>
          <div>{children}</div>
        </div>
      </div>
    </div>
  );
}
