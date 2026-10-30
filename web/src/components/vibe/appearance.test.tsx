import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AppearanceControl, VibeAppearance } from "./appearance";

let root: Root;
let container: HTMLDivElement;
let dark = false;
let systemChanged: () => void;
beforeEach(() => {
  globalThis.IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear(); dark = false;
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: dark, addEventListener: (_: string, notify: () => void) => { systemChanged = notify; }, removeEventListener: vi.fn() })));
  const nativeStyle = window.getComputedStyle.bind(window);
  vi.spyOn(window, "getComputedStyle").mockImplementation(element => {
    const style = nativeStyle(element);
    return new Proxy(style, { get(target, key) {
      if (key === "getPropertyValue") return (name: string) => name === "--vibe-bg" ? element.getAttribute("data-appearance") === "dark" ? "#12110f" : "#faf8f5" : target.getPropertyValue(name);
      return Reflect.get(target, key);
    } });
  });
  document.head.innerHTML = '<meta name="theme-color" media="(prefers-color-scheme: light)" content="#original-light"><meta name="theme-color" media="(prefers-color-scheme: dark)" content="#original-dark">';
  container = document.createElement("div"); document.body.append(container); root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); document.head.innerHTML = ""; vi.restoreAllMocks(); vi.unstubAllGlobals(); });
const colors = () => [...document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')].map(meta => meta.content);
function choose(value: string) {
  act(() => { const select = container.querySelector("select")!; select.value = value; select.dispatchEvent(new Event("change", { bubbles: true })); });
}

it("keeps the browser bar in sync with system changes and explicit appearance, then restores each meta", () => {
  act(() => root.render(<VibeAppearance><AppearanceControl /></VibeAppearance>));
  expect(colors()).toEqual(["#faf8f5", "#faf8f5"]);
  act(() => { dark = true; systemChanged(); });
  expect(colors()).toEqual(["#12110f", "#12110f"]);
  choose("light");
  expect(colors()).toEqual(["#faf8f5", "#faf8f5"]);
  act(() => { dark = false; systemChanged(); });
  expect(colors()).toEqual(["#faf8f5", "#faf8f5"]);
  choose("dark");
  expect(colors()).toEqual(["#12110f", "#12110f"]);
  choose("system");
  expect(colors()).toEqual(["#faf8f5", "#faf8f5"]);
  act(() => root.render(null));
  expect(colors()).toEqual(["#original-light", "#original-dark"]);
});

it("uses the saved appearance for the browser bar on mount", () => {
  localStorage.setItem("vibe-appearance", "dark");
  act(() => root.render(<VibeAppearance><AppearanceControl /></VibeAppearance>));
  expect(container.querySelector("main")?.getAttribute("data-appearance")).toBe("dark");
  expect(colors()).toEqual(["#12110f", "#12110f"]);
});
