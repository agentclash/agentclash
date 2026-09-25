"use client";

import { useId, useState } from "react";
import { VibeButton } from "./vibe-button";

export function RetryModelControl({ models, disabled, onRetry }: {
  models: { id: string; name: string }[];
  disabled: boolean;
  onRetry: (model: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const id = useId();
  if (!models.length) return null;
  return (
    <div>
      <VibeButton disabled={disabled} aria-expanded={open} aria-controls={id}
        onClick={() => setOpen(value => !value)}>
        Retry with another model
      </VibeButton>
      {open && (
        <div id={id} className="mt-3 space-y-2" role="group" aria-label="Choose a model to retry">
          <p className="vibe-muted">Choose a model to retry your saved message.</p>
          <div className="flex flex-wrap gap-2">
            {models.map(model => (
              <VibeButton key={model.id} disabled={disabled} onClick={() => {
                setOpen(false);
                onRetry(model.id);
              }}>
                {model.name}
              </VibeButton>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
