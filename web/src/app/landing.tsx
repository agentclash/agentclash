"use client";

import type React from "react";
import { useRef, useState } from "react";
import Link from "next/link";
import { useAuth } from "@workos-inc/authkit-nextjs/components";
import { ArrowRight, Calendar, ExternalLink, LogIn, Star } from "lucide-react";
import {
  Anthropic,
  Gemini,
  Mistral,
  OpenAI,
  OpenRouter,
  XAI,
} from "@lobehub/icons";

const DEMO_URL = "https://cal.com/atharva-kanherkar-epgztu/agentclash-demo";

function ClashMark({
  className = "",
  animated = false,
}: {
  className?: string;
  animated?: boolean;
}) {
  return (
    <svg
      viewBox="0 0 512 512"
      className={className}
      aria-label="AgentClash"
      role="img"
    >
      <g className={animated ? "animate-clash-left" : undefined}>
        <polygon
          points="80,180 240,256 80,332"
          fill="#ffffff"
          opacity="0.95"
        />
      </g>
      <g className={animated ? "animate-clash-right" : undefined}>
        <polygon
          points="432,180 272,256 432,332"
          fill="#ffffff"
          opacity="0.5"
        />
      </g>
      <g className={animated ? "animate-clash-sparks" : undefined}>
        <line
          x1="256" y1="96" x2="256" y2="168"
          stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75"
        />
        <line
          x1="256" y1="344" x2="256" y2="416"
          stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75"
        />
        <line
          x1="186" y1="130" x2="216" y2="188"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="326" y1="130" x2="296" y2="188"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="186" y1="382" x2="216" y2="324"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
        <line
          x1="326" y1="382" x2="296" y2="324"
          stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55"
        />
      </g>
    </svg>
  );
}

const PROVIDERS: Array<{ name: string; render: (size: number) => React.ReactNode }> = [
  { name: "OpenAI", render: (size) => <OpenAI size={size} color="#74AA9C" /> },
  { name: "Anthropic", render: (size) => <Anthropic size={size} color="#D97757" /> },
  { name: "Gemini", render: (size) => <Gemini.Color size={size} /> },
  { name: "xAI", render: (size) => <XAI size={size} color="#FFFFFF" /> },
  { name: "Mistral", render: (size) => <Mistral.Color size={size} /> },
  { name: "OpenRouter", render: (size) => <OpenRouter size={size} color="#6566F1" /> },
];

function DottedSpotlight({
  children,
  className = "",
}: {
  children?: React.ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLElement | null>(null);
  const [isActive, setIsActive] = useState(false);

  function updatePosition(clientX: number, clientY: number) {
    if (!ref.current) return;
    const rect = ref.current.getBoundingClientRect();
    ref.current.style.setProperty("--mx", `${clientX - rect.left}px`);
    ref.current.style.setProperty("--my", `${clientY - rect.top}px`);
  }

  const cursorMask =
    "radial-gradient(400px circle at var(--mx) var(--my), black 0%, black 20%, transparent 70%)";
  const dotImage =
    "radial-gradient(rgba(255,255,255,1) 1px, transparent 1px)";
  const cursorBloom =
    "radial-gradient(400px circle at var(--mx) var(--my), rgba(255,255,255,0.06) 0%, rgba(255,255,255,0.015) 35%, transparent 70%)";

  return (
    <section
      ref={ref}
      onMouseMove={(e) => {
        updatePosition(e.clientX, e.clientY);
        setIsActive(true);
      }}
      onMouseLeave={() => setIsActive(false)}
      onTouchStart={(e) => {
        const t = e.touches[0];
        if (!t) return;
        updatePosition(t.clientX, t.clientY);
        setIsActive(true);
      }}
      onTouchMove={(e) => {
        const t = e.touches[0];
        if (!t) return;
        updatePosition(t.clientX, t.clientY);
      }}
      onTouchEnd={() => setIsActive(false)}
      onTouchCancel={() => setIsActive(false)}
      className={`relative ${className}`}
      style={{ ["--mx" as string]: "50%", ["--my" as string]: "50%" }}
    >
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div
          aria-hidden
          className="animate-dots-breathe absolute inset-0"
          style={{
            backgroundImage: dotImage,
            backgroundSize: "22px 22px",
            maskImage:
              "radial-gradient(ellipse 40% 60% at 50% 50%, black 0%, black 25%, transparent 80%)",
            WebkitMaskImage:
              "radial-gradient(ellipse 40% 60% at 50% 50%, black 0%, black 25%, transparent 80%)",
          }}
        />
        <div
          aria-hidden
          className={`absolute inset-0 transition-opacity duration-500 ease-out ${isActive ? "opacity-100" : "opacity-0"}`}
          style={{ backgroundImage: cursorBloom }}
        />
        <div
          aria-hidden
          className={`absolute inset-0 transition-opacity duration-500 ease-out ${isActive ? "opacity-90" : "opacity-0"}`}
          style={{
            backgroundImage: dotImage,
            backgroundSize: "22px 22px",
            maskImage: cursorMask,
            WebkitMaskImage: cursorMask,
          }}
        />
      </div>
      <div className="relative">{children}</div>
    </section>
  );
}

function TargetGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden
    >
      <circle cx="24" cy="24" r="19" opacity="0.32" />
      <circle cx="24" cy="24" r="12" opacity="0.6" />
      <circle cx="24" cy="24" r="5" opacity="0.9" />
      <circle cx="24" cy="24" r="1.5" fill="currentColor" stroke="none" />
    </svg>
  );
}

function LineupGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="currentColor"
      aria-hidden
    >
      <polygon points="6,13 14,18 6,23" opacity="0.95" />
      <polygon points="20,13 28,18 20,23" opacity="0.8" />
      <polygon points="34,13 42,18 34,23" opacity="0.65" />
      <polygon points="6,28 14,33 6,38" opacity="0.5" />
      <polygon points="20,28 28,33 20,38" opacity="0.4" />
      <polygon points="34,28 42,33 34,38" opacity="0.3" />
    </svg>
  );
}

function LightFlowArrows() {
  const COUNT = 9;
  const DURATION = 3.6;
  return (
    <div
      className="flex flex-col items-center justify-center gap-5 py-8 sm:gap-7 sm:py-12"
      aria-hidden
    >
      {Array.from({ length: COUNT }).map((_, i) => (
        <svg
          key={i}
          viewBox="0 0 48 24"
          className="animate-arrow-flow h-5 w-11 text-white"
          style={{
            animationDelay: `${(-(i / COUNT) * DURATION).toFixed(2)}s`,
          }}
          focusable="false"
        >
          <path
            d="M6 7 L24 19 L42 7"
            stroke="currentColor"
            strokeWidth="2.25"
            strokeLinecap="round"
            strokeLinejoin="round"
            fill="none"
          />
        </svg>
      ))}
    </div>
  );
}

function TransparentFrame() {
  const ys = [82, 118, 154, 190, 226];
  const paths = ys.map((y) => `M -30 ${y} L 430 ${y}`);
  return (
    <div className="flex items-center justify-center py-4" aria-hidden>
      <svg
        viewBox="0 0 400 300"
        className="w-full max-w-[440px]"
        focusable="false"
      >
        <rect
          x="80"
          y="60"
          width="240"
          height="180"
          rx="2"
          fill="none"
          stroke="rgba(255,255,255,0.35)"
          strokeWidth="1.1"
        />
        <rect
          x="92"
          y="72"
          width="216"
          height="156"
          rx="1"
          fill="none"
          stroke="rgba(255,255,255,0.08)"
          strokeWidth="1"
          strokeDasharray="2 5"
        />

        {ys.map((y, i) => (
          <line
            key={`tf-track-${i}`}
            x1="-20"
            y1={y}
            x2="420"
            y2={y}
            stroke="rgba(255,255,255,0.05)"
            strokeWidth="1"
          />
        ))}

        {paths.map((d, i) => (
          <line
            key={`tf-streak-${i}`}
            x1="-7"
            y1="0"
            x2="7"
            y2="0"
            stroke="white"
            strokeWidth="2"
            strokeLinecap="round"
            className="animate-light-streak"
            style={{
              offsetPath: `path('${d}')`,
              animationDelay: `${(-(i / paths.length) * 1.4).toFixed(2)}s`,
            }}
          />
        ))}
      </svg>
    </div>
  );
}

function ToolGlyph({ name }: { name: string }) {
  const common = {
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.4,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (name) {
    case "read_file":
      return (
        <g {...common}>
          <path d="M 8 5 H 22 V 27 H 8 Z" />
          <line x1="11" y1="11" x2="19" y2="11" />
          <line x1="11" y1="15" x2="19" y2="15" />
          <line x1="11" y1="19" x2="17" y2="19" />
        </g>
      );
    case "write_file":
      return (
        <g {...common}>
          <path d="M 6 5 H 18 V 27 H 6 Z" />
          <path d="M 14 10 L 24 20 L 20 24 L 10 14 Z" />
          <line x1="22" y1="22" x2="26" y2="26" />
        </g>
      );
    case "search_text":
      return (
        <g {...common}>
          <circle cx="13" cy="13" r="7" />
          <line x1="19" y1="19" x2="25" y2="25" />
          <line x1="9" y1="12" x2="17" y2="12" strokeWidth="1" opacity="0.55" />
          <line x1="9" y1="15" x2="15" y2="15" strokeWidth="1" opacity="0.55" />
        </g>
      );
    case "query_sql":
      return (
        <g {...common}>
          <ellipse cx="16" cy="8" rx="9" ry="3" />
          <path d="M 7 8 V 24 A 9 3 0 0 0 25 24 V 8" />
          <path d="M 7 16 A 9 3 0 0 0 25 16" opacity="0.55" />
        </g>
      );
    case "http_request":
      return (
        <g {...common}>
          <circle cx="16" cy="16" r="11" />
          <line x1="5" y1="16" x2="27" y2="16" opacity="0.6" />
          <path d="M 16 5 Q 10 16 16 27 Q 22 16 16 5" opacity="0.7" />
        </g>
      );
    case "run_tests":
      return (
        <g {...common}>
          <rect x="5" y="5" width="22" height="22" rx="2" />
          <path d="M 10 16 L 14 20 L 22 11" strokeWidth="1.8" />
        </g>
      );
    case "build":
      return (
        <g {...common}>
          <path d="M 16 4 L 19 7 L 19 11 L 23 13 L 23 19 L 19 21 L 19 25 L 16 28 L 13 25 L 13 21 L 9 19 L 9 13 L 13 11 L 13 7 Z" />
          <circle cx="16" cy="16" r="3" />
        </g>
      );
    case "exec":
      return (
        <g {...common}>
          <rect x="4" y="7" width="24" height="18" rx="1.5" />
          <path d="M 10 13 L 14 16 L 10 19" strokeWidth="1.6" />
          <line x1="16" y1="19" x2="22" y2="19" strokeWidth="1.4" />
        </g>
      );
    case "submit":
      return (
        <g {...common}>
          <path d="M 16 24 L 16 6" strokeWidth="1.8" />
          <path d="M 9 13 L 16 6 L 23 13" strokeWidth="1.8" />
          <line x1="6" y1="28" x2="26" y2="28" strokeWidth="1.4" opacity="0.5" />
        </g>
      );
    default:
      return null;
  }
}

function ToolPalette() {
  const TOOLS = [
    "read_file",
    "write_file",
    "search_text",
    "query_sql",
    "http_request",
    "run_tests",
    "build",
    "exec",
    "submit",
  ];

  const labelProps = {
    fill: "white",
    opacity: 0.5,
    fontSize: 10,
    fontFamily: "var(--font-mono), monospace",
    letterSpacing: "0.14em",
    textTransform: "uppercase" as const,
  } as const;

  const ringStroke = {
    fill: "none",
    stroke: "rgba(255,255,255,0.28)",
    strokeWidth: 1.1,
  } as const;

  const LLM_YS = [60, 120, 180, 240, 300, 360];
  const LLM_CX = 36;
  const AGENT_CX = 624;
  const AGENT_YS = [80, 160, 260, 340];

  const MATRIX_X = 200;
  const MATRIX_Y = 90;
  const MATRIX_W = 260;
  const MATRIX_H = 260;
  const COL = [MATRIX_X + 35, MATRIX_X + 130, MATRIX_X + 225];
  const ROW = [MATRIX_Y + 40, MATRIX_Y + 130, MATRIX_Y + 220];

  const MERGE = { x: 172, y: 210 };
  const SPLIT = { x: 492, y: 210 };

  const convergePaths = LLM_YS.map((y) => `M 50 ${y} Q 100 ${y} ${MERGE.x} ${MERGE.y}`);
  const intoMatrix = `M ${MERGE.x} ${MERGE.y} L ${MATRIX_X - 2} ${MERGE.y}`;
  const outOfMatrix = `M ${MATRIX_X + MATRIX_W + 2} ${SPLIT.y} L ${SPLIT.x} ${SPLIT.y}`;
  const divergePaths = AGENT_YS.map(
    (y) => `M ${SPLIT.x} ${SPLIT.y} Q ${SPLIT.x + 50} ${y} ${AGENT_CX - 16} ${y}`,
  );

  const allFlowPaths = [
    ...convergePaths,
    intoMatrix,
    outOfMatrix,
    ...divergePaths,
  ];

  return (
    <div className="flex items-center justify-center py-4" aria-hidden>
      <svg
        viewBox="0 0 660 420"
        className="w-full max-w-[640px] text-white/75"
        focusable="false"
      >
        <defs>
          <marker
            id="tool-arrow"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="5"
            markerHeight="5"
            orient="auto"
          >
            <polygon points="0,0 10,5 0,10" fill="white" opacity="0.55" />
          </marker>
        </defs>

        <text x={LLM_CX} y="26" textAnchor="middle" {...labelProps}>
          llms
        </text>
        <text
          x={MATRIX_X + MATRIX_W / 2}
          y="26"
          textAnchor="middle"
          {...labelProps}
        >
          sandbox
        </text>
        <text x={AGENT_CX} y="26" textAnchor="middle" {...labelProps}>
          agents
        </text>

        {LLM_YS.map((y, i) => {
          const provider = PROVIDERS[i];
          if (!provider) return null;
          const size = 20;
          return (
            <g key={provider.name}>
              <circle cx={LLM_CX} cy={y} r="14" {...ringStroke} />
              <g transform={`translate(${LLM_CX - size / 2} ${y - size / 2})`}>
                {provider.render(size)}
              </g>
            </g>
          );
        })}

        <rect
          x={MATRIX_X}
          y={MATRIX_Y}
          width={MATRIX_W}
          height={MATRIX_H}
          rx="14"
          fill="none"
          stroke="rgba(255,255,255,0.18)"
          strokeWidth="1"
          strokeDasharray="2 4"
        />

        {TOOLS.map((name, i) => {
          const row = Math.floor(i / 3);
          const col = i % 3;
          return (
            <g
              key={name}
              transform={`translate(${COL[col] - 16} ${ROW[row] - 16})`}
            >
              <ToolGlyph name={name} />
            </g>
          );
        })}

        {AGENT_YS.map((y, i) => (
          <g key={`agent-${i}`}>
            <circle cx={AGENT_CX} cy={y} r="13" {...ringStroke} />
            <circle
              cx={AGENT_CX}
              cy={y}
              r="3.5"
              fill="rgba(255,255,255,0.35)"
            />
          </g>
        ))}

        {allFlowPaths.map((d, i) => (
          <path
            key={`tp-${i}`}
            d={d}
            fill="none"
            stroke="rgba(255,255,255,0.14)"
            strokeWidth="1.1"
            markerEnd="url(#tool-arrow)"
          />
        ))}

        {allFlowPaths.map((d, i) => (
          <line
            key={`ts-${i}`}
            x1="-7"
            y1="0"
            x2="7"
            y2="0"
            stroke="white"
            strokeWidth="2"
            strokeLinecap="round"
            className="animate-light-streak"
            style={{
              offsetPath: `path('${d}')`,
              animationDelay: `${(-(i / allFlowPaths.length) * 1.4).toFixed(2)}s`,
            }}
          />
        ))}
      </svg>
    </div>
  );
}

function ScoringPipeline() {
  const labelProps = {
    fill: "#888888",
    opacity: 0.8,
    fontSize: 10,
    fontFamily: "var(--font-mono), monospace",
    letterSpacing: "0.1em",
    textTransform: "uppercase" as const,
  } as const;

  const ringStroke = {
    fill: "rgba(255,255,255,0.02)",
    stroke: "rgba(255,255,255,0.22)",
    strokeWidth: 1.2,
  } as const;

  const AGENT_X = 60;
  const AGENT_YS = [40, 105, 170, 235, 300, 365];
  const AGENT_R = 16;

  const JUDGE_X = 300;
  const JUDGES = [
    { y: 85, label: "deterministic" },
    { y: 165, label: "mathematic" },
    { y: 245, label: "behavioural" },
    { y: 325, label: "llm + aggregation" },
  ];
  const JUDGE_R = 16;

  // 6 agents → 4 judges by nearest y, then 4 judges → verdict.
  const paths = [
    "M 76 40  Q 180 60  282 85",
    "M 76 105 Q 180 95  282 85",
    "M 76 170 Q 180 167 282 165",
    "M 76 235 Q 180 240 282 245",
    "M 76 300 Q 180 315 282 325",
    "M 76 365 Q 180 345 282 325",
    "M 318 85  Q 420 130 506 190",
    "M 318 165 Q 420 180 506 202",
    "M 318 245 Q 420 220 506 212",
    "M 318 325 Q 420 270 506 222",
  ];

  return (
    <div className="flex items-center justify-center py-4" aria-hidden>
      <svg
        viewBox="0 0 600 440"
        className="w-full max-w-[580px]"
        focusable="false"
      >
        <defs>
          <filter id="soft-glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="6" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>

        <text
          x="24"
          y="210"
          textAnchor="middle"
          transform="rotate(-90 24 210)"
          {...labelProps}
        >
          agents
        </text>
        <text x="300" y="46" textAnchor="middle" {...labelProps}>
          judges
        </text>
        <text x="540" y="150" textAnchor="middle" {...labelProps}>
          verdict
        </text>

        {AGENT_YS.map((y, i) => {
          const provider = PROVIDERS[i];
          if (!provider) return null;
          const iconSize = 22;
          return (
            <g key={provider.name}>
              <circle cx={AGENT_X} cy={y} r={AGENT_R} {...ringStroke} />
              <g transform={`translate(${AGENT_X - iconSize / 2} ${y - iconSize / 2})`}>
                {provider.render(iconSize)}
              </g>
            </g>
          );
        })}

        {JUDGES.map((j) => (
          <g key={j.label}>
            <circle
              cx={JUDGE_X}
              cy={j.y}
              r={JUDGE_R}
              {...ringStroke}
              filter="url(#soft-glow)"
            />
            <text
              x={JUDGE_X + JUDGE_R + 10}
              y={j.y + 4}
              textAnchor="start"
              fill="white"
              opacity="0.55"
              fontSize="10.5"
              fontFamily="var(--font-mono), monospace"
              letterSpacing="0.04em"
            >
              {j.label}
            </text>
          </g>
        ))}

        <circle
          cx="540"
          cy="205"
          r="30"
          {...ringStroke}
          stroke="rgba(255,255,255,0.55)"
          strokeWidth="1.5"
          filter="url(#soft-glow)"
          className="animate-results-glow"
        />

        {paths.map((d, i) => (
          <path
            key={`p-${i}`}
            d={d}
            fill="none"
            stroke="rgba(255,255,255,0.15)"
            strokeWidth="1.2"
          />
        ))}

        {paths.map((d, i) => (
          <line
            key={`g-${i}`}
            x1="-7"
            y1="0"
            x2="7"
            y2="0"
            stroke="white"
            strokeWidth="2"
            strokeLinecap="round"
            className="animate-light-streak"
            style={{
              offsetPath: `path('${d}')`,
              animationDelay: `${(-(i / paths.length) * 1.4).toFixed(2)}s`,
            }}
          />
        ))}
      </svg>
    </div>
  );
}

function SandboxLanes() {
  const DURATION = 3.8;
  return (
    <div
      className="flex flex-col items-stretch justify-center gap-3.5 py-6 sm:gap-4 sm:py-10"
      aria-hidden
    >
      {PROVIDERS.map(({ name, render }, i) => (
        <div
          key={name}
          className="relative h-12 overflow-hidden rounded-md border border-white/[0.14]"
        >
          <div
            className="animate-sandbox-travel absolute inset-0 flex items-center justify-center"
            style={{
              animationDelay: `${(-(i / PROVIDERS.length) * DURATION).toFixed(2)}s`,
            }}
          >
            {render(24)}
          </div>
        </div>
      ))}
    </div>
  );
}

function FeedbackLoop() {
  return (
    <div className="flex items-center justify-center py-6 sm:py-10" aria-hidden>
      <svg
        viewBox="0 0 400 230"
        className="w-full max-w-[480px]"
        focusable="false"
      >
        <defs>
          <marker
            id="feedback-arrow-head"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto"
          >
            <polygon points="0,0 10,5 0,10" fill="white" opacity="0.7" />
          </marker>
        </defs>

        <circle
          cx="70"
          cy="115"
          r="30"
          fill="none"
          stroke="white"
          strokeWidth="1.2"
          opacity="0.7"
        />
        <circle
          cx="330"
          cy="115"
          r="30"
          fill="none"
          stroke="white"
          strokeWidth="1.2"
          opacity="0.7"
        />

        <path
          d="M 92 99 Q 200 10 308 99"
          stroke="white"
          strokeWidth="1.25"
          fill="none"
          opacity="0.42"
          markerEnd="url(#feedback-arrow-head)"
        />
        <circle r="3.2" fill="white" className="animate-travel-top" />

        <path
          d="M 308 131 Q 200 220 92 131"
          stroke="white"
          strokeWidth="1.25"
          fill="none"
          opacity="0.42"
          markerEnd="url(#feedback-arrow-head)"
        />
        <circle r="3.2" fill="white" className="animate-travel-bottom" />
      </svg>
    </div>
  );
}

function TrackGlyph() {
  return (
    <svg
      viewBox="0 0 48 48"
      className="size-7 text-white/90"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      aria-hidden
    >
      <line x1="5" y1="24" x2="34" y2="24" opacity="0.45" />
      <circle cx="10" cy="24" r="1.3" fill="currentColor" opacity="0.4" stroke="none" />
      <circle cx="18" cy="24" r="1.3" fill="currentColor" opacity="0.65" stroke="none" />
      <circle cx="26" cy="24" r="2" fill="currentColor" opacity="0.95" stroke="none" />
      <line x1="36" y1="12" x2="36" y2="36" strokeWidth="1.2" opacity="0.55" />
      <g fill="currentColor" stroke="none" opacity="0.9">
        <rect x="37" y="12" width="3" height="3" />
        <rect x="40" y="15" width="3" height="3" />
        <rect x="37" y="18" width="3" height="3" />
        <rect x="40" y="21" width="3" height="3" />
        <rect x="37" y="24" width="3" height="3" />
      </g>
    </svg>
  );
}

export default function HomePage() {
  const { user, loading: authLoading } = useAuth();

  return (
    <main className="min-h-screen flex flex-col">
      {/* ── Header ──────────────────────────────────────────────── */}
      <header className="px-8 sm:px-12 py-6 border-b border-white/[0.06]">
        <div className="mx-auto flex max-w-[1440px] items-center justify-between">
          <Link
            href="/"
            className="inline-flex items-center gap-2.5 text-white/90"
          >
            <ClashMark className="size-6" />
            <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
              AgentClash
            </span>
          </Link>
          <nav className="flex items-center gap-1 sm:gap-2 text-xs">
            <Link
              href="/docs"
              className="inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
            >
              Docs
            </Link>
            <Link
              href="/blog"
              className="hidden sm:inline-flex px-3 py-1.5 text-white/55 hover:text-white/85 transition-colors"
            >
              Blog
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-white/60 hover:text-white/85 hover:border-white/15 transition-colors"
            >
              <Star className="size-3.5" />
              GitHub
            </a>
            {authLoading ? (
              <span className="inline-flex h-[30px] w-[88px] rounded-md border border-white/[0.08] bg-white/[0.04]" />
            ) : user ? (
              <Link
                href="/dashboard"
                className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 font-medium text-[#060606] hover:bg-white/90 transition-colors"
              >
                Dashboard
                <ArrowRight className="size-3" />
              </Link>
            ) : (
              <Link
                href="/auth/login"
                className="inline-flex items-center gap-1.5 rounded-md border border-white/15 bg-white/[0.04] px-3 py-1.5 text-white/75 hover:text-white hover:border-white/25 transition-colors"
              >
                <LogIn className="size-3.5" />
                Sign in
              </Link>
            )}
          </nav>
        </div>
      </header>

      {/* ── Hero ────────────────────────────────────────────────── */}
      <DottedSpotlight className="px-8 sm:px-12 pt-32 pb-20 sm:pt-44 sm:pb-28">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-[1.5fr_1fr] md:gap-20 items-center">
          <div>
            <h1 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.95] text-[clamp(3rem,7vw,7.5rem)] max-w-[16ch]">
              Ship the right agent.
              <br />
              <span className="text-white/40">Not the loudest one.</span>
            </h1>

            <p className="mt-10 max-w-[44ch] text-lg sm:text-xl leading-[1.5] text-white/55">
              AgentClash races your models head-to-head on real tasks. Same
              challenge, same tools, same time budget — scored live across
              completion, speed, and efficiency.
            </p>

            <div className="mt-10 flex flex-col sm:flex-row sm:flex-wrap sm:items-center gap-3">
              {user ? (
                <>
                  <Link
                    href="/dashboard"
                    className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                  >
                    Go to dashboard
                    <ArrowRight className="size-4" />
                  </Link>
                  <a
                    href="https://github.com/agentclash/agentclash"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                  >
                    <Star className="size-4" />
                    View on GitHub
                  </a>
                </>
              ) : (
                <>
                  <a
                    href={DEMO_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                  >
                    <Calendar className="size-4" />
                    Book a demo
                  </a>
                  <Link
                    href="/auth/login"
                    className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                  >
                    Get started
                    <ArrowRight className="size-4" />
                  </Link>
                  <a
                    href="https://github.com/agentclash/agentclash"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-6 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
                  >
                    <Star className="size-4" />
                    GitHub
                  </a>
                </>
              )}
            </div>
          </div>

          <div className="flex items-center justify-center">
            <div className="flex aspect-square w-full max-w-[260px] md:max-w-[520px] items-center justify-center mx-auto">
              <ClashMark
                animated
                className="w-full max-w-[200px] md:max-w-[360px] aspect-square"
              />
            </div>
          </div>
        </div>
      </DottedSpotlight>

      {/* ── Feature · Replay ────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              Scrub the replay. See exactly where it got stuck.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.6] text-white/55">
              Every think, every tool call, every observation is captured.
              Step back to the moment a model went sideways — the prompt
              it saw, the output it produced, the state it worked from. No
              more guessing why one model won and another flunked.
            </p>
          </div>
          <div>
            <LightFlowArrows />
          </div>
        </div>
      </section>

      {/* ── Providers ───────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <div className="flex flex-col gap-10 md:flex-row md:items-end md:justify-between md:gap-16">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[20ch]">
              Any model.
              <br />
              <span className="text-white/40">Any provider.</span>
            </h2>
            <p className="max-w-[42ch] text-base leading-[1.6] text-white/50">
              Normalised tool-calls, normalised errors, same scoring rules.
              First-class adapters for the providers below, plus OpenRouter
              for the long tail — three hundred more models, no extra code.
            </p>
          </div>

          <ul className="mt-20 grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-px border-y border-white/[0.06] bg-white/[0.06]">
            {PROVIDERS.map(({ name, render }, i) => (
              <li
                key={name}
                className="group relative flex flex-col items-center justify-center gap-4 overflow-hidden bg-[#060606] py-14 transition-colors hover:bg-white/[0.02]"
              >
                <div
                  aria-hidden
                  className="animate-provider-glow pointer-events-none absolute left-1/2 top-[44%] size-32 -translate-x-1/2 -translate-y-1/2 rounded-full transition-opacity duration-500 group-hover:opacity-[0.8]"
                  style={{
                    background:
                      "radial-gradient(circle, rgba(255,255,255,0.18), transparent 70%)",
                    animationDelay: `${(-(i / PROVIDERS.length) * 9).toFixed(2)}s`,
                  }}
                />
                <div className="relative opacity-90 transition-opacity group-hover:opacity-100">
                  {render(40)}
                </div>
                <span className="relative text-sm text-white/55 transition-colors group-hover:text-white/85">
                  {name}
                </span>
              </li>
            ))}
          </ul>

          <p className="mt-8 text-sm text-white/40">
            Plus 300 more via OpenRouter. New first-class providers landing
            every month.
          </p>
        </div>
      </section>

      {/* ── Sandbox ─────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              A fresh microVM for every agent.
            </h2>
            <p className="mt-10 max-w-[48ch] text-lg leading-[1.6] text-white/60">
              Each racer boots into its own Firecracker microVM — isolated
              filesystem, isolated network, no shared kernel. When the race
              ends, the sandbox is torn down. The next one spins up clean.
            </p>
            <p className="mt-6 max-w-[48ch] text-lg leading-[1.6] text-white/60">
              That isolation isn&apos;t just safety. It&apos;s what makes
              the race fair. No model gets a warm cache. No prompt leaks
              between lanes. The only variable in the race is the model.
            </p>
            <p className="mt-10 max-w-[48ch] text-sm text-white/40">
              Powered by{" "}
              <a
                href="https://e2b.dev"
                target="_blank"
                rel="noopener noreferrer"
                className="text-white/65 underline decoration-white/20 underline-offset-4 transition-colors hover:text-white/90 hover:decoration-white/40"
              >
                E2B
              </a>
              &nbsp;— the sandbox infrastructure behind AI products at
              Perplexity, Hugging Face, and Groq.
            </p>
          </div>
          <div>
            <SandboxLanes />
          </div>
        </div>
      </section>

      {/* ── Tool use ────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              Real tools. Real effects.
            </h2>
            <p className="mt-10 max-w-[50ch] text-lg leading-[1.6] text-white/60">
              Agents race with the same primitives a developer uses —
              file I/O, data queries, HTTP, shell, test runners. Real
              commands, real sandboxed effects, not a transcript of
              imagined tool calls.
            </p>

            <p className="mt-10 max-w-[54ch] text-base leading-[1.6] text-white/55">
              <span className="text-white/80">Compose your own.</span>{" "}
              Challenge packs define higher-level tools —{" "}
              <code className="font-[family-name:var(--font-mono)] text-white/75">
                inventory_lookup
              </code>
              ,{" "}
              <code className="font-[family-name:var(--font-mono)] text-white/75">
                migrate_db
              </code>
              , whatever your domain needs — that wrap the primitives
              with templated arguments. Pack manifest, not an SDK.
            </p>
            <p className="mt-6 max-w-[54ch] text-sm text-white/45">
              Fine-grained policy per pack: allowed tool kinds, shell
              access, network access, max calls per run. Benchmark under
              tight constraints, or unlock full-power for dev races.
            </p>
          </div>
          <div>
            <ToolPalette />
          </div>
        </div>
      </section>

      {/* ── Scoring ─────────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.25rem,5vw,4.5rem)] max-w-[20ch]">
              One number is a lie.
            </h2>
            <p className="mt-10 max-w-[50ch] text-lg leading-[1.6] text-white/60">
              Every run is judged from four independent vantage points,
              with consensus aggregation across multiple judge models.
              One composite verdict per eval session. Weights you control.
            </p>

            <dl className="mt-10 grid gap-x-10 gap-y-5 sm:grid-cols-2">
              <div>
                <dt className="text-[15px] font-medium text-white/90">
                  Deterministic
                </dt>
                <dd className="mt-1 text-[13px] leading-[1.55] text-white/50">
                  exact, regex, JSON Schema, code execution, file-tree
                  assertions.
                </dd>
              </div>
              <div>
                <dt className="text-[15px] font-medium text-white/90">
                  Mathematic
                </dt>
                <dd className="mt-1 text-[13px] leading-[1.55] text-white/50">
                  math equivalence, BLEU, ROUGE, ChrF, token F1, numeric
                  tolerance.
                </dd>
              </div>
              <div>
                <dt className="text-[15px] font-medium text-white/90">
                  Behavioural
                </dt>
                <dd className="mt-1 text-[13px] leading-[1.55] text-white/50">
                  recovery, exploration, scope adherence, confidence
                  calibration &middot; plus latency, cost, reliability.
                </dd>
              </div>
              <div>
                <dt className="text-[15px] font-medium text-white/90">
                  LLM + aggregation
                </dt>
                <dd className="mt-1 text-[13px] leading-[1.55] text-white/50">
                  rubric, assertion, reference, pairwise &middot; median,
                  mean, majority-vote, unanimous consensus.
                </dd>
              </div>
            </dl>

            <p className="mt-10 max-w-[52ch] text-sm text-white/45">
              Grounded in a decade of open evaluation research. We
              didn&apos;t invent the primitives; we wired them together
              so you can run them all in one eval session.
            </p>
            <p className="mt-4 text-sm text-white/40">
              Combined weighted, binary, or hybrid-with-gates — tuned to
              the bar you&apos;d ship against.
            </p>
          </div>
          <div>
            <ScoringPipeline />
          </div>
        </div>
      </section>

      {/* ── Feature · Regression tests ──────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,4rem)]">
              Failures become your regression suite.
            </h2>
            <div className="mt-10 space-y-6">
              <p className="text-lg leading-[1.6] text-white/60">
                When a model flunks a challenge, the failing trace is
                frozen into a permanent test. Next week&apos;s race
                replays it. The following month&apos;s does too.
              </p>
              <p className="text-lg leading-[1.6] text-white/60">
                Your eval suite sharpens itself with use. By the time a
                new model arrives, it walks into a track that was paved
                by every mistake the last model made.
              </p>
            </div>
          </div>
          <div>
            <FeedbackLoop />
          </div>
        </div>
      </section>

      {/* ── How it works ────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <div className="flex flex-col gap-10 md:flex-row md:items-end md:justify-between md:gap-16">
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[20ch]">
              From challenge to scoreboard.
            </h2>
            <p className="max-w-[38ch] text-base leading-[1.6] text-white/50">
              Set up a head-to-head race in under a minute. Watch a verdict
              arrive in the time it takes to finish a coffee.
            </p>
          </div>

          <div className="relative mt-24">
            <div
              className="hidden md:block pointer-events-none absolute left-0 right-0 top-[32px] border-t border-dashed border-white/10"
              aria-hidden
            />

            <div
              className="hidden md:block pointer-events-none absolute left-0 right-0 top-[30px] h-[4px] overflow-hidden"
              aria-hidden
            >
              {[0, 1].map((i) => (
                <div
                  key={i}
                  className="animate-steps-streak absolute top-[1px] h-[2px] w-12 rounded-full bg-white"
                  style={{
                    animationDelay: `${(-(i / 2) * 4).toFixed(2)}s`,
                  }}
                />
              ))}
            </div>

            {/* Mobile vertical connector — dashed line + streaks running
                top to bottom through the stacked step rings. */}
            <div
              className="md:hidden pointer-events-none absolute top-[32px] bottom-0 left-[31px] border-l border-dashed border-white/10"
              aria-hidden
            />
            <div
              className="md:hidden pointer-events-none absolute top-0 bottom-0 left-[30px] w-[4px] overflow-hidden"
              aria-hidden
            >
              {[0, 1].map((i) => (
                <div
                  key={i}
                  className="animate-steps-streak-vertical absolute left-[1px] w-[2px] h-12 rounded-full bg-white"
                  style={{
                    animationDelay: `${(-(i / 2) * 4).toFixed(2)}s`,
                  }}
                />
              ))}
            </div>

            <ol className="relative grid gap-20 md:grid-cols-3 md:gap-14">
              {[
                {
                  n: "01",
                  title: "Pick a challenge",
                  body:
                    "Write your own or pull from the library. Real tasks — a broken auth server, a SQL bug, a spec to implement — not trivia.",
                  glyph: <TargetGlyph />,
                },
                {
                  n: "02",
                  title: "Pick your models",
                  body:
                    "Line up six or eight contestants across providers. Same tool policy, same time budget, same starting state.",
                  glyph: <LineupGlyph />,
                },
                {
                  n: "03",
                  title: "Watch them race",
                  body:
                    "Live scoring as they work. Composite metric across completion, speed, token efficiency, and tool strategy.",
                  glyph: <TrackGlyph />,
                },
              ].map((step) => (
                <li key={step.n} className="relative">
                  <div className="relative z-10 inline-flex size-16 items-center justify-center rounded-full border border-white/15 bg-[#060606]">
                    {step.glyph}
                  </div>
                  <p className="mt-10 font-[family-name:var(--font-display)] text-6xl leading-none tracking-[-0.03em] text-white/15">
                    {step.n}
                  </p>
                  <h3 className="mt-4 font-[family-name:var(--font-display)] text-3xl sm:text-4xl tracking-[-0.02em] leading-[1.08] text-white/95">
                    {step.title}
                  </h3>
                  <p className="mt-5 max-w-[34ch] text-base leading-[1.65] text-white/55">
                    {step.body}
                  </p>
                </li>
              ))}
            </ol>
          </div>
        </div>
      </section>

      {/* ── Why we built this ───────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[22ch]">
            We got tired of being lied to.
          </h2>

          <div className="mt-14 max-w-[58ch] space-y-7 text-lg leading-[1.65] text-white/65">
            <p>
              A few months ago we were picking a model for a production
              agent — the kind that reads a ticket, opens a PR, runs the
              tests, writes a comment. The benchmarks said one thing. MMLU
              said another. Vendor blog posts told a third. We ran our own
              evals; they were flaky and painful to reason about. We picked
              a model. A week in, it started failing on the exact shape of
              ticket we&apos;d built it for — the same shape it had passed
              every eval we threw at it.
            </p>
            <p>
              We re-read every score. None of them had touched our task.
              They had measured one kind of intelligence, and we had
              shipped another.
            </p>
            <p className="font-[family-name:var(--font-display)] text-2xl sm:text-3xl leading-[1.3] tracking-[-0.015em] text-white/90 !mt-12">
              Static benchmarks leak. Leaderboards reward hype. The only
              eval you can trust is the one you ran yourself, on your own
              task, against every other model you were considering, at the
              same time.
            </p>
            <p>
              AgentClash is what we wish had existed that week. You
              describe the task the way your product actually does it. Pick
              six models. They race, live, on the same inputs, with the
              same tools, scored on what matters in production —
              correctness, cost, latency, behaviour under pressure. When
              one fails, the failing trace becomes a test. Every mistake
              ratchets the eval tighter.
            </p>
            <p>
              We&apos;re building it in the open because no closed
              benchmark has ever stayed honest for long. If this feels
              familiar — run a race. Your task. Your models. Your
              scoreboard.
            </p>
          </div>
        </div>
      </section>

      {/* ── Closing CTA ─────────────────────────────────────────── */}
      <section className="border-t border-white/[0.06] px-8 sm:px-12 py-40 sm:py-56">
        <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
          <div>
            <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.95] text-[clamp(2.75rem,6vw,6rem)] max-w-[16ch]">
              Stop guessing.
              <br />
              <span className="text-white/40">Start racing.</span>
            </h2>
            <div className="mt-10 flex flex-col sm:flex-row sm:flex-wrap gap-3">
              {user ? (
                <Link
                  href="/dashboard"
                  className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                >
                  Go to dashboard
                  <ArrowRight className="size-4" />
                </Link>
              ) : (
                <>
                  <a
                    href={DEMO_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
                  >
                    <Calendar className="size-4" />
                    Book a demo
                  </a>
                  <Link
                    href="/auth/login"
                    className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
                  >
                    Start your first race
                    <ArrowRight className="size-4" />
                  </Link>
                </>
              )}
              <a
                href="https://github.com/agentclash/agentclash"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.02] px-7 py-3 text-sm font-medium text-white/60 hover:text-white/90 hover:border-white/20 transition-colors"
              >
                <Star className="size-4" />
                Star on GitHub
                <ExternalLink className="size-3.5 text-white/40" />
              </a>
            </div>
            <div className="mt-12 max-w-[46ch] border-t border-white/[0.08] pt-8">
              <p className="font-[family-name:var(--font-display)] text-xl sm:text-2xl tracking-[-0.015em] leading-[1.3] text-white/85">
                An eval engine you can&apos;t audit isn&apos;t an eval
                engine.
              </p>
              <p className="mt-3 text-sm text-white/45">
                Open source. FSL-1.1-MIT. Read the code, fork it,
                self-host it.
              </p>
            </div>
          </div>
          <div>
            <TransparentFrame />
          </div>
        </div>
      </section>

      {/* ── Footer ──────────────────────────────────────────────── */}
      <footer className="mt-auto border-t border-white/[0.06] px-8 sm:px-12 py-10">
        <div className="mx-auto max-w-[1440px] flex flex-wrap items-center justify-between gap-4 text-[11px] font-[family-name:var(--font-mono)] text-white/35">
          <div className="flex items-center gap-6">
            <span className="font-medium text-white/55">AgentClash</span>
            <span>FSL-1.1-MIT</span>
          </div>
          <div className="flex items-center gap-5">
            <Link href="/blog" className="hover:text-white/70 transition-colors">
              Blog
            </Link>
            <Link href="/team" className="hover:text-white/70 transition-colors">
              Team
            </Link>
            <a
              href="https://github.com/agentclash/agentclash"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-white/70 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </main>
  );
}
