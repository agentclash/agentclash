import type { ReactNode } from "react";

type Props = {
  eyebrow?: string;
  title: ReactNode;
  body?: ReactNode;
  align?: "left" | "center";
  size?: "md" | "lg";
  className?: string;
};

export function SectionHeading({
  eyebrow,
  title,
  body,
  align = "left",
  size = "md",
  className = "",
}: Props) {
  const alignClass = align === "center" ? "mx-auto text-center" : "";
  const titleSize =
    size === "lg"
      ? "text-[clamp(2.75rem,6.5vw,6rem)]"
      : "text-[clamp(2.25rem,5vw,4.5rem)]";

  return (
    <div className={`${alignClass} ${className}`}>
      {eyebrow ? (
        <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
          <span className="inline-block size-1 rounded-full bg-white/60" />
          {eyebrow}
        </p>
      ) : null}
      <h2
        className={`font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] ${titleSize} max-w-[22ch] ${
          align === "center" ? "mx-auto" : ""
        }`}
      >
        {title}
      </h2>
      {body ? (
        <div
          className={`mt-8 max-w-[52ch] text-lg leading-[1.6] text-white/55 ${
            align === "center" ? "mx-auto" : ""
          }`}
        >
          {body}
        </div>
      ) : null}
    </div>
  );
}
