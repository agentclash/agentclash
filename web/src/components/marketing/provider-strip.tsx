import type { ReactNode } from "react";
import {
  Anthropic,
  Gemini,
  Mistral,
  OpenAI,
  OpenRouter,
  XAI,
} from "@lobehub/icons";

type Provider = { name: string; render: (size: number) => ReactNode };

// aria-hidden keeps lobehub's hardcoded SVG <title> out of the accessibility
// tree so crawlers do not treat "OpenAI" as the page title when metadata is
// still streaming.
const PROVIDERS: Provider[] = [
  {
    name: "OpenAI",
    render: (size) => <OpenAI size={size} color="#74AA9C" aria-hidden />,
  },
  {
    name: "Anthropic",
    render: (size) => <Anthropic size={size} color="#D97757" aria-hidden />,
  },
  {
    name: "Gemini",
    render: (size) => <Gemini.Color size={size} aria-hidden />,
  },
  {
    name: "xAI",
    render: (size) => <XAI size={size} color="#FFFFFF" aria-hidden />,
  },
  {
    name: "Mistral",
    render: (size) => <Mistral.Color size={size} aria-hidden />,
  },
  {
    name: "OpenRouter",
    render: (size) => <OpenRouter size={size} color="#6566F1" aria-hidden />,
  },
];

export function ProviderStrip() {
  return (
    <ul className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-px border-y border-white/[0.06] bg-white/[0.06]">
      {PROVIDERS.map(({ name, render }) => (
        <li
          key={name}
          className="flex flex-col items-center justify-center gap-3 bg-[#060606] px-6 py-10 text-white/70"
        >
          <div className="flex h-10 items-center justify-center">
            {render(32)}
          </div>
          <p className="text-2xs font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/40">
            {name}
          </p>
        </li>
      ))}
    </ul>
  );
}
