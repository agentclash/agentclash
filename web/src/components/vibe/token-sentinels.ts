// Test helper. In getComputedStyle, jsdom 31 replaces each var() with the value of its custom property.
// For this reason, a computed value does not show the token name. This helper gives each design token a unique value.
// When a computed value is equal to one of these values, it shows the token that the rule uses.
export const tokenSentinels = {
  "--vibe-font-heading": "vibe-font-heading",
  "--vibe-font-data": "vibe-font-data",
  "--vibe-font-footnote": "101px",
  "--vibe-font-callout": "102px",
  "--vibe-font-display-s": "103px",
  "--vibe-fg": "rgb(0, 0, 101)",
  "--vibe-muted": "rgb(0, 0, 102)",
  "--vibe-ink": "rgb(0, 0, 103)",
  "--vibe-issue": "rgb(0, 0, 104)",
  "--vibe-text-3": "rgb(0, 0, 105)",
} as const;

// Inline values on the element replace the token values from the stylesheets. The child elements inherit the inline values.
export function setTokenSentinels(element: HTMLElement) {
  for (const [name, value] of Object.entries(tokenSentinels)) element.style.setProperty(name, value);
}
