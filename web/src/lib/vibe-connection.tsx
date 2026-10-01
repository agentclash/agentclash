"use client";

import { createContext, useContext } from "react";

// Shared transport/configuration only, never a duplicate session or run store.
export const VibeConnection = createContext<{ token: () => Promise<string | null | undefined>; contact?: { email: string; available: boolean } }>({ token: async () => undefined });
export const useVibeConnection = () => useContext(VibeConnection);
