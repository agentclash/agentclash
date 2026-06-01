"use client";

import {
  Children,
  isValidElement,
  useMemo,
  type ReactNode,
} from "react";
import { CodeBlock } from "@/components/replay/agent-output-renderer";

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

function languageFromPreChildren(children: ReactNode): string | undefined {
  for (const child of Children.toArray(children)) {
    if (!isValidElement<{ className?: string }>(child)) continue;
    const match = child.props.className?.match(/language-([\w-]+)/);
    if (match?.[1]) return match[1];
  }
  return undefined;
}

export function CopyableCodeBlock({ children }: { children: ReactNode }) {
  const text = useMemo(() => textFromNode(children).trimEnd(), [children]);
  const language = useMemo(
    () => languageFromPreChildren(children),
    [children],
  );

  if (language) {
    return (
      <CodeBlock
        code={text}
        language={language}
        showLineNumbers={text.split("\n").length > 1}
      />
    );
  }

  return (
    <div className="not-prose my-6 overflow-x-auto rounded-2xl border border-white/[0.08] bg-white/[0.04] p-4">
      <pre className="m-0 font-mono text-[13px] leading-6 text-white/78">
        <code>{text}</code>
      </pre>
    </div>
  );
}
