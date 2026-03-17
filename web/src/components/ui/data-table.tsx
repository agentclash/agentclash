"use client";

import { ReactNode } from "react";

type DataTableProps = {
  title: string;
  contextLabel?: string;
  columns: { key: string; label: string; align?: "left" | "right" }[];
  rows: Record<string, ReactNode>[];
  footer?: { label: string; value: ReactNode };
};

export function DataTable({ title, contextLabel, columns, rows, footer }: DataTableProps) {
  return (
    <div className="rounded-[10px] border border-border overflow-hidden font-[family-name:var(--font-mono)] text-[13px]">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-b border-border">
        <div className="flex items-center gap-2">
          <div className="flex gap-[5px]">
            <span className="w-2 h-2 rounded-full bg-text-4" />
            <span className="w-2 h-2 rounded-full bg-text-4" />
            <span className="w-2 h-2 rounded-full bg-text-4" />
          </div>
          <span className="text-[11px] text-text-3 ml-2">{title}</span>
        </div>
        {contextLabel && (
          <span className="text-[11px] text-text-4">{contextLabel}</span>
        )}
      </div>

      {/* Column headers */}
      <div
        className="grid border-b border-border px-4 py-1.5"
        style={{ gridTemplateColumns: columns.map(() => "1fr").join(" ") }}
      >
        {columns.map((col) => (
          <span
            key={col.key}
            className="text-[10px] font-medium uppercase tracking-[0.06em] text-text-4"
            style={{ textAlign: col.align || "left" }}
          >
            {col.label}
          </span>
        ))}
      </div>

      {/* Rows */}
      {rows.map((row, i) => (
        <div
          key={i}
          className="grid px-4 py-[7px] border-b border-border last:border-b-0"
          style={{ gridTemplateColumns: columns.map(() => "1fr").join(" ") }}
        >
          {columns.map((col) => (
            <span
              key={col.key}
              className="tabular-nums"
              style={{ textAlign: col.align || "left" }}
            >
              {row[col.key]}
            </span>
          ))}
        </div>
      ))}

      {/* Footer */}
      {footer && (
        <div className="flex items-center justify-between px-4 py-2.5 bg-surface border-t border-border">
          <span className="text-[11px] text-text-3">{footer.label}</span>
          <span className="text-[11px] font-semibold">{footer.value}</span>
        </div>
      )}
    </div>
  );
}
