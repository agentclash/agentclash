"use client";

import { useLayoutEffect, type RefObject } from "react";

export function useComposerAutosize(
  ref: RefObject<HTMLTextAreaElement | null>,
  content: string,
  compact: boolean,
  visible: boolean,
  placement: string,
) {
  useLayoutEffect(() => {
    const input = ref.current;
    if (!input || !visible) return;
    let disposed = false;
    let width = input.getBoundingClientRect().width;
    const resize = () => {
      if (disposed) return;
      input.style.height = "auto";
      input.style.height = `${Math.min(input.scrollHeight, compact ? 160 : 240)}px`;
    };
    resize();
    const observer = typeof ResizeObserver === "undefined" ? undefined : new ResizeObserver(() => {
      const nextWidth = input.getBoundingClientRect().width;
      if (nextWidth === width) return;
      width = nextWidth;
      resize();
    });
    observer?.observe(input);
    void document.fonts?.ready.then(resize);
    return () => { disposed = true; observer?.disconnect(); };
  }, [ref, content, compact, visible, placement]);
}
