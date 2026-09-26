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
      // A wrapped hint is not user content. It must not expand an empty input.
      const placeholder = input.placeholder;
      input.placeholder = "";
      const viewport = window.visualViewport?.height || window.innerHeight;
      input.style.height = `${Math.min(input.scrollHeight, compact ? 160 : 240, Math.max(44, viewport * 0.24))}px`;
      input.placeholder = placeholder;
    };
    resize();
    const observer = typeof ResizeObserver === "undefined" ? undefined : new ResizeObserver(() => {
      const nextWidth = input.getBoundingClientRect().width;
      if (nextWidth === width) return;
      width = nextWidth;
      resize();
    });
    observer?.observe(input);
    window.visualViewport?.addEventListener("resize", resize);
    void document.fonts?.ready.then(resize);
    return () => { disposed = true; observer?.disconnect(); window.visualViewport?.removeEventListener("resize", resize); };
  }, [ref, content, compact, visible, placement]);
}
