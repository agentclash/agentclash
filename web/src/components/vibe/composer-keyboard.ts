import type { KeyboardEvent } from "react";

export function sendOnEnter(
  event: KeyboardEvent<HTMLTextAreaElement>,
  send: () => void,
) {
  if (
    event.key !== "Enter" ||
    event.shiftKey ||
    event.nativeEvent.isComposing ||
    event.keyCode === 229
  )
    return;
  const touch = typeof window.matchMedia === "function" && window.matchMedia("(any-pointer: coarse)").matches;
  if (touch && !event.metaKey && !event.ctrlKey) return;
  event.preventDefault();
  send();
}
