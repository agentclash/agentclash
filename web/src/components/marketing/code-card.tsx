type Props = {
  title?: string;
  language?: string;
  code: string;
};

export function CodeCard({ title, language = "shell", code }: Props) {
  return (
    <div className="rounded-lg border border-white/[0.08] bg-white/[0.02] overflow-hidden">
      <div className="flex items-center justify-between border-b border-white/[0.06] px-4 py-2.5">
        <span className="text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] text-white/40">
          {title ?? language}
        </span>
        <div className="flex items-center gap-1.5">
          <span className="size-2 rounded-full bg-white/15" />
          <span className="size-2 rounded-full bg-white/15" />
          <span className="size-2 rounded-full bg-white/15" />
        </div>
      </div>
      <pre className="overflow-x-auto px-5 py-4 font-[family-name:var(--font-mono)] text-[13px] leading-[1.7] text-white/75">
        <code>{code}</code>
      </pre>
    </div>
  );
}
