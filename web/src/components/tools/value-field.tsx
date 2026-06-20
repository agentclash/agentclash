"use client";

import { useRef } from "react";
import { ChevronDown, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { controlClass } from "./field";
import type { ValueReference } from "./lib/friendly";

/**
 * A single-line value input that a non-engineer can fill without learning the
 * `${...}` template syntax. They type literal text, then use the "Insert value"
 * menu to drop in an agent input, an earlier step's result, or a secret — the
 * token is spliced at the cursor and the syntax stays hidden.
 */
export function ValueField({
  value,
  onChange,
  placeholder,
  references,
  allowSecret = false,
  ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  references: ValueReference[];
  allowSecret?: boolean;
  ariaLabel?: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  // Where to splice an inserted token. Tracked so the menu (which steals focus)
  // can still insert at the spot the user last had their cursor.
  const caret = useRef<{ start: number; end: number }>({
    start: value.length,
    end: value.length,
  });

  function rememberCaret() {
    const el = inputRef.current;
    if (!el) return;
    caret.current = {
      start: el.selectionStart ?? value.length,
      end: el.selectionEnd ?? value.length,
    };
  }

  function insertAtCaret(token: string, selectInsideFrom?: string) {
    const { start, end } = caret.current;
    const next = value.slice(0, start) + token + value.slice(end);
    onChange(next);
    // Restore focus and place the caret sensibly after React re-renders.
    requestAnimationFrame(() => {
      const el = inputRef.current;
      if (!el) return;
      el.focus();
      if (selectInsideFrom && token.includes(selectInsideFrom)) {
        const selStart = start + token.indexOf(selectInsideFrom);
        el.setSelectionRange(selStart, selStart + selectInsideFrom.length);
      } else {
        const pos = start + token.length;
        el.setSelectionRange(pos, pos);
      }
      caret.current = {
        start: el.selectionStart ?? next.length,
        end: el.selectionEnd ?? next.length,
      };
    });
  }

  const groups = groupReferences(references);
  const hasMenu = references.length > 0 || allowSecret;

  return (
    <div className="flex items-stretch gap-1.5">
      <input
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onSelect={rememberCaret}
        onKeyUp={rememberCaret}
        onClick={rememberCaret}
        onBlur={rememberCaret}
        placeholder={placeholder}
        aria-label={ariaLabel}
        className={cn(controlClass, "font-[family-name:var(--font-mono)] text-xs")}
      />
      {hasMenu && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button type="button" variant="outline" size="sm" className="shrink-0" />}
          >
            <Plus data-icon="inline-start" className="size-3.5" />
            Insert
            <ChevronDown data-icon="inline-end" className="size-3.5 opacity-60" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="max-h-72 w-64 overflow-auto">
            {groups.length === 0 && !allowSecret ? (
              <DropdownMenuLabel>Nothing to insert yet</DropdownMenuLabel>
            ) : (
              groups.map((g, gi) => (
                <DropdownMenuGroup key={g.group}>
                  {gi > 0 && <DropdownMenuSeparator />}
                  <DropdownMenuLabel>{g.group}</DropdownMenuLabel>
                  {g.items.map((ref) => (
                    <DropdownMenuItem
                      key={ref.token}
                      onClick={() => insertAtCaret(ref.token)}
                    >
                      {ref.label}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuGroup>
              ))
            )}
            {allowSecret && (
              <>
                {groups.length > 0 && <DropdownMenuSeparator />}
                <DropdownMenuLabel>Secrets</DropdownMenuLabel>
                <DropdownMenuItem
                  onClick={() => insertAtCaret("${secrets.NAME}", "NAME")}
                >
                  A stored secret…
                </DropdownMenuItem>
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}

function groupReferences(
  references: ValueReference[],
): { group: string; items: ValueReference[] }[] {
  const order: string[] = [];
  const byGroup = new Map<string, ValueReference[]>();
  for (const ref of references) {
    if (!byGroup.has(ref.group)) {
      byGroup.set(ref.group, []);
      order.push(ref.group);
    }
    byGroup.get(ref.group)!.push(ref);
  }
  return order.map((group) => ({ group, items: byGroup.get(group)! }));
}
