export function badgeMarkdown(slug: string, baseUrl = "https://www.agentclash.dev/try"): string {
  const origin = baseUrl.replace(/\/try$/, "");
  return `[![Try on AgentClash](${origin}/api/try/badge/${slug}.svg)](${baseUrl}/${slug})`;
}

export function badgeSvg(label = "Try in Terminal"): string {
  const width = Math.max(140, label.length * 7 + 50);
  return `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="20" role="img" aria-label="${label}">
  <linearGradient id="g" x2="0" y2="100%">
    <stop offset="0" stop-color="#1a1a2e"/>
    <stop offset="1" stop-color="#16213e"/>
  </linearGradient>
  <rect width="${width}" height="20" rx="3" fill="url(#g)"/>
  <rect x="0" width="22" height="20" rx="3" fill="#0f3460"/>
  <rect x="19" width="3" height="20" fill="#0f3460"/>
  <text x="11" y="14" fill="#e94560" font-family="monospace" font-size="11" text-anchor="middle">&gt;_</text>
  <text x="28" y="14" fill="#eaeaea" font-family="system-ui,sans-serif" font-size="11">${label}</text>
</svg>`;
}
