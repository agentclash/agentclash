import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { Breadcrumbs, type BreadcrumbEntry } from "@/components/ui/breadcrumbs";

export type { BreadcrumbEntry };

interface PageHeaderProps {
  title: string;
  breadcrumbs?: BreadcrumbEntry[];
  actions?: ReactNode;
  className?: string;
}

export function PageHeader({
  title,
  breadcrumbs,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <div className={cn("flex flex-col gap-2 pb-6", className)}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumbs entries={breadcrumbs} />
      )}
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-lg font-semibold tracking-tight">{title}</h1>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </div>
  );
}
