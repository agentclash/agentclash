export function ClashMark({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 512 512"
      className={className}
      aria-label="AgentClash"
      role="img"
    >
      <polygon points="80,180 240,256 80,332" fill="#ffffff" opacity="0.95" />
      <polygon points="432,180 272,256 432,332" fill="#ffffff" opacity="0.5" />
      <g>
        <line x1="256" y1="96" x2="256" y2="168" stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75" />
        <line x1="256" y1="344" x2="256" y2="416" stroke="#ffffff" strokeWidth="10" strokeLinecap="round" opacity="0.75" />
        <line x1="186" y1="130" x2="216" y2="188" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55" />
        <line x1="326" y1="130" x2="296" y2="188" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55" />
        <line x1="186" y1="382" x2="216" y2="324" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55" />
        <line x1="326" y1="382" x2="296" y2="324" stroke="#ffffff" strokeWidth="8" strokeLinecap="round" opacity="0.55" />
      </g>
    </svg>
  );
}
