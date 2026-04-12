"use client";

import { cn } from "@/lib/utils";

interface JsonFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  disabled?: boolean;
  rows?: number;
  description?: string;
}

export function JsonField({
  label,
  value,
  onChange,
  error,
  disabled,
  rows = 6,
  description,
}: JsonFieldProps) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">{label}</label>
      {description && (
        <p className="mb-1.5 text-xs text-muted-foreground">{description}</p>
      )}
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        rows={rows}
        spellCheck={false}
        className={cn(
          "block w-full rounded-lg border bg-transparent px-3 py-2 font-[family-name:var(--font-mono)] text-xs leading-relaxed placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y disabled:opacity-50 disabled:cursor-not-allowed",
          error
            ? "border-destructive focus:border-destructive focus:ring-destructive/50"
            : "border-input focus:border-ring",
        )}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  );
}
