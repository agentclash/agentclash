import { Fragment } from "react";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";

export interface BreadcrumbEntry {
  label: string;
  href?: string;
}

/**
 * Single breadcrumb implementation shared across the app. Replaces the
 * previously hand-rolled, inconsistent breadcrumbs on run / scorecard / replay
 * pages so every trail looks and behaves the same.
 */
export function Breadcrumbs({
  entries,
  className,
}: {
  entries: BreadcrumbEntry[];
  className?: string;
}) {
  if (entries.length === 0) return null;
  return (
    <Breadcrumb className={className}>
      <BreadcrumbList>
        {entries.map((crumb, i) => {
          const isLast = i === entries.length - 1;
          return (
            <Fragment key={`${crumb.label}-${i}`}>
              <BreadcrumbItem>
                {isLast || !crumb.href ? (
                  <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                ) : (
                  <BreadcrumbLink href={crumb.href}>{crumb.label}</BreadcrumbLink>
                )}
              </BreadcrumbItem>
              {!isLast && <BreadcrumbSeparator />}
            </Fragment>
          );
        })}
      </BreadcrumbList>
    </Breadcrumb>
  );
}
