import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AgentReply, SafeMarkdown, safeLink } from "./safe-markdown";
describe("Vibe untrusted rendering", () => {
  it("does not render HTML, scripts, remote images or javascript links", () => {
    const html = renderToStaticMarkup(
      <SafeMarkdown>
        {
          '<script>alert(1)</script>\n\n<img src="https://evil.invalid/pixel">\n\n![pixel](https://evil.invalid/track)\n\n[click](javascript:alert%281%29)'
        }
      </SafeMarkdown>,
    );
    expect(html).not.toContain("<script");
    expect(html).not.toContain("<img");
    expect(html).not.toContain('href="javascript:');
  });
  it("rejects protocol-relative and data links", () => {
    expect(safeLink("//attacker.invalid")).toBe("");
    expect(safeLink("data:text/html,hi")).toBe("");
    expect(safeLink("https://example.com")).toBe("https://example.com");
  });
  it("shows structured replies verbatim without rounding numbers or interpreting HTML", () => {
    const reply =
      '{"id":9007199254740993,"text":"<script>alert(1)</script>","escaped":"\\n"}';
    const html = renderToStaticMarkup(<AgentReply>{reply}</AgentReply>);
    const container = document.createElement("div");
    container.innerHTML = html;
    expect(container.querySelector("pre code")?.textContent).toBe(reply);
    expect(container.querySelector("script")).toBeNull();
  });
});
