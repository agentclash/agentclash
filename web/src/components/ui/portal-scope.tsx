"use client";
import { createContext, useContext } from "react";
// Optional styling for a route's complete portal tree, including its backdrop.
// Other application routes retain the components' existing defaults.
export const PortalScope = createContext<{ className?: string; "data-appearance"?: "light" | "dark" }>({});
export const usePortalScope = () => useContext(PortalScope);
