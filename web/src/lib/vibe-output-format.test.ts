import { expect, it } from "vitest";
import { jsonOutput, outputDownload } from "./vibe-output-format";

it("preserves raw JSON bytes, including large numbers", () => {
  const original = ' { "id": 900719925474099312345, "value": "\\u263A" }\n';
  expect(jsonOutput(original)).toBe(original);
  expect(outputDownload(original, "json").contents).toBe(original);
  expect(outputDownload(original, "markdown").contents).toBe(original);
});

it("removes only a single complete JSON fence and keeps payload bytes", () => {
  const payload = '{\r\n  "id": 900719925474099312345\r\n}\r\n';
  const original = `  \r\n\`\`\`json\r\n${payload}\`\`\`\r\n `;
  expect(jsonOutput(original)).toBe(payload);
  expect(outputDownload(original, "json").contents).toBe(payload);
  expect(outputDownload(original, "markdown").contents).toBe(original);
});

it("rejects prose, multiple fences, invalid JSON and plain text", () => {
  for (const text of ["hello", "Here it is:\n```json\n{}\n```", "```json\n{}\n```\n```json\n{}\n```", "```json\n{bad}\n```", "```js\n{}\n```"]) {
    expect(jsonOutput(text)).toBeUndefined();
    expect(() => outputDownload(text, "json")).toThrow("complete JSON");
  }
});
