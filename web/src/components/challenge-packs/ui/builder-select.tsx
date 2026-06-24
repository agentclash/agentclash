"use client";

// The builder's single dropdown. Replaces native <select>, which rendered an
// unreadable white system popup inside the dark app. We own the trigger styling
// (built from the base primitive to avoid the shared SelectTrigger's fixed
// height) and reuse the portaled, accessible popup parts from ui/select.

import { Select as SelectPrimitive } from "@base-ui/react/select";
import { ChevronDownIcon } from "lucide-react";

import { SelectContent, SelectGroup, SelectItem, SelectLabel } from "@/components/ui/select";
import { cn } from "@/lib/utils";

export type BuilderSelectOption = { value: string; label: string; disabled?: boolean };
export type BuilderSelectGroup = { label: string; options: BuilderSelectOption[] };

const triggerClass =
  "flex w-full items-center justify-between gap-2 rounded-md border border-builder-border bg-builder-surface px-3 py-2 text-left text-sm text-builder-fg transition-colors select-none data-placeholder:text-builder-fg-faint hover:border-builder-border-strong focus:border-builder-fg-muted focus:bg-builder-surface-hover focus:outline-none disabled:cursor-not-allowed disabled:opacity-50";

export function BuilderSelect({
  value,
  onChange,
  options,
  groups,
  placeholder,
  ariaLabel,
  id,
  disabled,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  options?: BuilderSelectOption[];
  groups?: BuilderSelectGroup[];
  placeholder?: string;
  ariaLabel?: string;
  id?: string;
  disabled?: boolean;
  className?: string;
}) {
  const renderItem = (o: BuilderSelectOption) => (
    <SelectItem key={o.value} value={o.value} disabled={o.disabled}>
      {o.label}
    </SelectItem>
  );

  return (
    <SelectPrimitive.Root
      value={value}
      onValueChange={(v) => {
        if (v != null) onChange(v as string);
      }}
    >
      <SelectPrimitive.Trigger
        id={id}
        aria-label={ariaLabel}
        disabled={disabled}
        className={cn(triggerClass, className)}
      >
        <span className="min-w-0 truncate">
          <SelectPrimitive.Value placeholder={placeholder} />
        </span>
        <SelectPrimitive.Icon
          render={<ChevronDownIcon className="size-4 shrink-0 text-builder-fg-subtle" />}
        />
      </SelectPrimitive.Trigger>
      <SelectContent
        alignItemWithTrigger={false}
        className="border border-builder-border bg-builder-panel"
      >
        {groups
          ? groups.map((g) => (
              <SelectGroup key={g.label}>
                <SelectLabel>{g.label}</SelectLabel>
                {g.options.map(renderItem)}
              </SelectGroup>
            ))
          : options?.map(renderItem)}
      </SelectContent>
    </SelectPrimitive.Root>
  );
}
