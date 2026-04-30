"use client";

import { Children, isValidElement, useMemo, useState, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";

function textFromNode(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") {
        return String(child);
      }

      if (isValidElement<{ children?: ReactNode }>(child)) {
        return textFromNode(child.props.children);
      }

      return "";
    })
    .join("");
}

export function CopyableCodeBlock({ children }: { children: ReactNode }) {
  const [copied, setCopied] = useState(false);
  const text = useMemo(() => textFromNode(children).trimEnd(), [children]);

  async function copyCode() {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return (
    <div className="group relative">
      <button
        type="button"
        onClick={copyCode}
        aria-label="Copy code"
        className="absolute right-3 top-3 z-10 inline-flex items-center gap-1.5 rounded-md border border-zinc-700 bg-zinc-950/90 px-2.5 py-1 text-xs font-medium text-zinc-300 opacity-100 shadow-sm transition-colors hover:border-emerald-500/60 hover:text-white sm:opacity-0 sm:group-hover:opacity-100"
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
        {copied ? "Copied" : "Copy"}
      </button>
      <pre>{children}</pre>
    </div>
  );
}
