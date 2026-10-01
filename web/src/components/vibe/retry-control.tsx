"use client";

import { useEffect, useState } from "react";
import { VibeButton } from "./vibe-button";

export function retryWait(availableAt?: string, serverTime?: string) {
  if (!availableAt) return 0;
  const now = serverTime ? Date.parse(serverTime) : Date.now();
  const remaining = Date.parse(availableAt) - now;
  return Number.isFinite(remaining) ? Math.max(0, remaining) : 0;
}

// Count down from a server snapshot using monotonic elapsed time. A wrong Mac
// clock must not unblock the action early. Expiry only enables a manual click.
export function RetryControl({ availableAt, serverTime, disabled, primary, label, onRetry }: {
  availableAt?: string;
  serverTime?: string;
  disabled: boolean;
  primary?: boolean;
  label: string;
  onRetry: () => void;
}) {
  const [remaining, setRemaining] = useState(() => retryWait(availableAt, serverTime));
  useEffect(() => {
    const start = performance.now();
    const wait = retryWait(availableAt, serverTime);
    const timer = setInterval(() => {
      const next = Math.max(0, wait - (performance.now() - start));
      setRemaining(next);
      if (!next) clearInterval(timer);
    }, 250);
    return () => clearInterval(timer);
  }, [availableAt, serverTime]);
  const seconds = Math.ceil(remaining / 1000);
  const waitLabel = seconds >= 60 ? `${Math.floor(seconds / 60)}m ${seconds % 60}s` : `${seconds}s`;
  return (
    <VibeButton variant={primary ? "primary" : "secondary"} disabled={disabled || remaining > 0} onClick={onRetry}>
      {remaining > 0 ? `Try again in ${waitLabel}` : label}
    </VibeButton>
  );
}
