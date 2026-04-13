import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

const sizeMap = {
  sm: "size-4",
  md: "size-6",
  lg: "size-8",
} as const;

interface LoadingSpinnerProps {
  size?: keyof typeof sizeMap;
  className?: string;
}

export function LoadingSpinner({ size = "md", className }: LoadingSpinnerProps) {
  return (
    <div role="status" aria-label="Loading" className={cn("flex items-center justify-center", className)}>
      <Loader2 className={cn("animate-spin text-muted-foreground", sizeMap[size])} />
    </div>
  );
}
