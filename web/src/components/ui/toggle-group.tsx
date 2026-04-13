"use client";

import { cn } from "@/lib/utils";

interface ToggleGroupOption<T extends string> {
  value: T;
  label: string;
  description?: string;
}

interface ToggleGroupProps<T extends string> {
  options: ToggleGroupOption<T>[];
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
}

export function ToggleGroup<T extends string>({
  options,
  value,
  onChange,
  disabled,
}: ToggleGroupProps<T>) {
  return (
    <div className="flex gap-2">
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => onChange(opt.value)}
          disabled={disabled}
          className={cn(
            "flex-1 rounded-lg border px-3 py-2 text-sm transition-colors disabled:opacity-50",
            value === opt.value
              ? "border-primary bg-primary/10 text-foreground"
              : "border-input text-muted-foreground hover:text-foreground",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
