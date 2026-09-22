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
  event.preventDefault();
  send();
}
