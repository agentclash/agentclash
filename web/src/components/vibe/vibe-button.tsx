"use client";

import { Button as Primitive } from "@base-ui/react/button";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

export function VibeButton({
  variant = "secondary",
  loading,
  className,
  children,
  ...props
}: Primitive.Props & {
  variant?: "primary" | "secondary" | "quiet";
  loading?: boolean;
}) {
  return (
    <Primitive
      {...props}
      className={cn("vibe-button", `vibe-button-${variant}`, className)}
      aria-busy={loading || undefined}
    >
      {loading && (
        <Loader2
          aria-hidden="true"
          className="size-4 animate-spin motion-reduce:animate-none"
        />
      )}
      {children}
    </Primitive>
  );
}
