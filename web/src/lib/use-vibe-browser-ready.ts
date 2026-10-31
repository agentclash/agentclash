"use client";
import { useSyncExternalStore } from "react";

const subscribe = () => () => {};
const browserSnapshot = () => true;
const serverSnapshot = () => false;
// Hydration uses the server defaults before any tab-local state is read.
export function useVibeBrowserReady() {
  return useSyncExternalStore(subscribe, browserSnapshot, serverSnapshot);
}
