import { ReactNode } from "react";

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4 mb-8">
      <div>
        {eyebrow && (
          <div className="flex items-center gap-2.5 mb-3">
            <span className="block w-7 h-[1.5px] bg-ds-accent" />
            <span className="text-xs font-medium tracking-[0.12em] uppercase text-ds-accent">
              {eyebrow}
            </span>
          </div>
        )}
        <h1 className="font-[family-name:var(--font-display)] text-[clamp(1.6rem,3.5vw,2.4rem)] leading-[1.1] text-text-1">
          {title}
        </h1>
        {description && (
          <p className="mt-2 text-sm text-text-2 max-w-lg">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2 shrink-0">{actions}</div>}
    </div>
  );
}
