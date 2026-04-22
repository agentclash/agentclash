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
      "border-lime-300/25 bg-lime-300/[0.08] text-lime-50/90 [&_strong]:text-lime-50",
  },
  warning: {
    icon: AlertTriangle,
    label: "Warning",
    className:
      "border-amber-300/25 bg-amber-300/[0.08] text-amber-50/90 [&_strong]:text-amber-50",
  },
  note: {
    icon: NotebookPen,
    label: "Note",
    className:
      "border-sky-300/20 bg-sky-300/[0.07] text-sky-50/90 [&_strong]:text-sky-50",
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
      className={`my-6 rounded-2xl border px-4 py-4 sm:px-5 ${style.className}`}
    >
      <div className="flex items-start gap-3">
        <Icon className="mt-0.5 size-4 shrink-0" />
        <div className="min-w-0 text-sm leading-7">
          <p className="mb-1 font-medium tracking-[-0.01em]">{style.label}</p>
          <div>{children}</div>
        </div>
      </div>
    </div>
  );
}
