export type OutputFormat = "markdown" | "json";

/** The JSON payload is a slice of the original reply, never a re-serialized value. */
export function jsonOutput(text: string): string | undefined {
  try { JSON.parse(text); return text; } catch { /* It may be one complete JSON fence. */ }
  const match = /^\s*```json[ \t]*\r?\n([\s\S]*\r?\n)```[ \t]*\s*$/i.exec(text);
  if (!match) return undefined;
  try { JSON.parse(match[1]); return match[1]; } catch { return undefined; }
}

export function outputDownload(text: string, format: OutputFormat): { contents: string; filename: string; type: string } {
  if (format === "json") {
    const contents = jsonOutput(text);
    if (contents === undefined) throw new Error("This output is not a complete JSON document.");
    return { contents, filename: "agent-output.json", type: "application/json;charset=utf-8" };
  }
  return { contents: text, filename: "agent-output.md", type: "text/markdown;charset=utf-8" };
}
