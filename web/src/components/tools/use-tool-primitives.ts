"use client";

import { useApiListQuery } from "@/lib/api/swr";
import type { ToolPrimitive } from "./lib/types";

/** Loads the static base-primitive catalog (GET /v1/tool-primitives). */
export function useToolPrimitives() {
  const { data, error, isLoading } = useApiListQuery<ToolPrimitive>(
    "/v1/tool-primitives",
    undefined,
    { revalidateOnFocus: false, dedupingInterval: 60_000 },
  );
  return {
    primitives: data?.items ?? [],
    error,
    isLoading,
  };
}
