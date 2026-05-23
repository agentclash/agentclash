"use client";

import { useState, useMemo } from "react";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import { Copy, Check, FileCode } from "lucide-react";

interface CodeBlockProps {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
}

const languageMap: Record<string, string> = {
  py: "python",
  js: "javascript",
  ts: "typescript",
  jsx: "jsx",
  tsx: "tsx",
  sh: "bash",
  bash: "bash",
  shell: "bash",
  zsh: "bash",
  yml: "yaml",
  yaml: "yaml",
  json: "json",
  md: "markdown",
  go: "go",
  rs: "rust",
  java: "java",
  cpp: "cpp",
  c: "c",
  cs: "csharp",
  rb: "ruby",
  php: "php",
  sql: "sql",
  html: "html",
  css: "css",
  xml: "xml",
  dockerfile: "docker",
  terraform: "hcl",
  tf: "hcl",
};

function normalizeLanguage(lang: string): string {
  const normalized = lang.toLowerCase().trim();
  return languageMap[normalized] || normalized || "text";
}

export function CodeBlock({ code, language = "text", showLineNumbers = true }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const normalizedLang = normalizeLanguage(language);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard unavailable (non-HTTPS or permission denied) — fail silently
    }
  };

  return (
    <div className="group relative my-3 overflow-hidden rounded-lg border border-white/[0.08] bg-[#1e1e1e]/95">
      {/* Header bar */}
      <div className="flex items-center justify-between border-b border-white/[0.06] bg-black/30 px-3 py-2">
        <div className="flex items-center gap-2">
          <FileCode className="size-3.5 text-white/40" />
          <span className="text-[11px] font-medium uppercase tracking-wider text-white/50">
            {normalizedLang}
          </span>
        </div>
        <button
          onClick={handleCopy}
          className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-white/5 px-2 py-1 text-[11px] text-white/60 transition-colors hover:bg-white/10 hover:text-white/90"
        >
          {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
          {copied ? "Copied" : "Copy"}
        </button>
      </div>

      {/* Code */}
      <div className="max-h-[400px] overflow-auto">
        <SyntaxHighlighter
          language={normalizedLang}
          style={vscDarkPlus}
          showLineNumbers={showLineNumbers}
          lineNumberStyle={{
            minWidth: "2.5em",
            paddingRight: "1em",
            color: "rgba(255,255,255,0.25)",
            fontSize: "12px",
            fontFamily: "var(--font-mono), ui-monospace, monospace",
          }}
          customStyle={{
            margin: 0,
            padding: "1rem",
            background: "transparent",
            fontSize: "13px",
            lineHeight: "1.6",
            fontFamily: "var(--font-mono), ui-monospace, monospace",
          }}
          codeTagProps={{
            style: {
              fontFamily: "var(--font-mono), ui-monospace, monospace",
              fontSize: "13px",
            },
          }}
        >
          {code}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}

/* ---------- Inline elements ---------- */

function InlineCode({ children }: { children: string }) {
  return (
    <code className="mx-0.5 inline rounded bg-white/[0.08] px-1.5 py-0.5 text-[0.9em] font-[family-name:var(--font-mono)] text-amber-300/90">
      {children}
    </code>
  );
}

function FilePath({ path }: { path: string }) {
  return (
    <span className="mx-0.5 inline-flex items-center gap-1 rounded bg-cyan-500/[0.08] px-1.5 py-0.5 text-[0.9em] font-[family-name:var(--font-mono)] text-cyan-300/90">
      <FileCode className="size-3 text-cyan-400/60" />
      {path}
    </span>
  );
}

function ShellCommand({ cmd }: { cmd: string }) {
  return (
    <span className="mx-0.5 inline rounded bg-emerald-500/[0.08] px-1.5 py-0.5 text-[0.9em] font-[family-name:var(--font-mono)] text-emerald-300/90">
      {cmd}
    </span>
  );
}

/* ---------- Parser ---------- */

type Token =
  | { type: "text"; value: string }
  | { type: "code_block"; language: string; code: string }
  | { type: "inline_code"; value: string }
  | { type: "file_path"; value: string }
  | { type: "shell_cmd"; value: string };

function detectLanguageFromContent(code: string): string {
  const trimmed = code.trim();
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) return "json";
  if (trimmed.startsWith("import ") || trimmed.startsWith("from ") || trimmed.includes("def ")) return "python";
  if (trimmed.startsWith("function ") || trimmed.includes("const ") || trimmed.includes("let ")) return "javascript";
  if (trimmed.includes("package main") || trimmed.includes("func ")) return "go";
  if (trimmed.startsWith("<!DOCTYPE") || trimmed.startsWith("<html")) return "html";
  if (trimmed.startsWith("SELECT ") || trimmed.startsWith("INSERT ") || trimmed.startsWith("CREATE ")) return "sql";
  if (trimmed.includes("#!/bin/bash") || trimmed.includes("#!/bin/sh")) return "bash";
  return "text";
}

function tokenize(text: string): Token[] {
  const tokens: Token[] = [];
  const codeBlockRegex = /```(\w+)?\n([\s\S]*?)```/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = codeBlockRegex.exec(text)) !== null) {
    // Text before the code block
    if (match.index > lastIndex) {
      tokens.push(...tokenizeInline(text.slice(lastIndex, match.index)));
    }
    // Code block
    tokens.push({
      type: "code_block",
      language: match[1] || detectLanguageFromContent(match[2]),
      code: match[2].trimEnd(),
    });
    lastIndex = match.index + match[0].length;
  }

  // Remaining text after last code block
  if (lastIndex < text.length) {
    tokens.push(...tokenizeInline(text.slice(lastIndex)));
  }

  if (tokens.length === 0) {
    tokens.push(...tokenizeInline(text));
  }

  return tokens;
}

function tokenizeInline(text: string): Token[] {
  const tokens: Token[] = [];
  // Regex to match: inline code `...`, file paths, or normal text
  // We process sequentially to handle overlapping patterns safely
  const regex = /(`[^`]+`)|(\/\w+(?:\/\w+)*(?:\.\w+)?)|([\$#]\s*\w+[^\n]*)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      tokens.push({ type: "text", value: text.slice(lastIndex, match.index) });
    }

    if (match[1]) {
      // Inline code `...`
      tokens.push({ type: "inline_code", value: match[1].slice(1, -1) });
    } else if (match[2]) {
      // File path /path/to/file
      const path = match[2];
      // Only style it as a path if it looks like one (has extension or is a known pattern)
      if (/\.\w{1,6}$/.test(path) || path.includes("/")) {
        tokens.push({ type: "file_path", value: path });
      } else {
        tokens.push({ type: "text", value: path });
      }
    } else if (match[3]) {
      // Shell command $ ... or # ...
      tokens.push({ type: "shell_cmd", value: match[3] });
    }

    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    tokens.push({ type: "text", value: text.slice(lastIndex) });
  }

  if (tokens.length === 0) {
    tokens.push({ type: "text", value: text });
  }

  return tokens;
}

/* ---------- Main renderer ---------- */

interface AgentOutputRendererProps {
  text: string;
}

export function AgentOutputRenderer({ text }: AgentOutputRendererProps) {
  // If the entire text looks like JSON, render it as a single JSON code block
  const parsedJson = useMemo(() => {
    const trimmed = text.trim();
    if ((trimmed.startsWith("{") && trimmed.endsWith("}")) || (trimmed.startsWith("[") && trimmed.endsWith("]"))) {
      try {
        return JSON.parse(trimmed);
      } catch {
        return null;
      }
    }
    return null;
  }, [text]);

  const tokens = useMemo(() => {
    if (parsedJson) return [];
    return tokenize(text);
  }, [text, parsedJson]);

  if (parsedJson) {
    return (
      <CodeBlock
        code={JSON.stringify(parsedJson, null, 2)}
        language="json"
        showLineNumbers={false}
      />
    );
  }

  return (
    <div className="text-[13px] leading-relaxed text-white/85">
      {tokens.map((token, i) => {
        switch (token.type) {
          case "code_block":
            return <CodeBlock key={i} code={token.code} language={token.language} />;
          case "inline_code":
            return <InlineCode key={i}>{token.value}</InlineCode>;
          case "file_path":
            return <FilePath key={i} path={token.value} />;
          case "shell_cmd":
            return <ShellCommand key={i} cmd={token.value} />;
          case "text":
          default:
            return <span key={i}>{token.value}</span>;
        }
      })}
    </div>
  );
}
