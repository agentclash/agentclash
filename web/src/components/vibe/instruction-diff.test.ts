import { expect, it } from "vitest";
import { lineDiff } from "./instruction-diff";

it.each([
  ["no change", "One\nTwo", "One\nTwo", [{ kind: "same", text: "One" }, { kind: "same", text: "Two" }]],
  ["one added line", "One\nThree", "One\nTwo\nThree", [{ kind: "same", text: "One" }, { kind: "added", text: "Two" }, { kind: "same", text: "Three" }]],
  ["one removed line", "One\nTwo", "One", [{ kind: "same", text: "One" }, { kind: "removed", text: "Two" }]],
  ["one changed line", "Old", "New", [{ kind: "removed", text: "Old" }, { kind: "added", text: "New" }]],
  ["empty before", "", "New\nRule", [{ kind: "added", text: "New" }, { kind: "added", text: "Rule" }]],
  ["both empty", "", "", []],
  ["empty after", "Old", "", [{ kind: "removed", text: "Old" }]],
  ["repeated lines", "A\nB\nA", "A\nA", [{ kind: "same", text: "A" }, { kind: "removed", text: "B" }, { kind: "same", text: "A" }]],
  ["whitespace and final newline", "  A\n", " A\n", [{ kind: "removed", text: "  A" }, { kind: "added", text: " A" }, { kind: "same", text: "" }]],
])("diffs lines: %s", (_name, before, after, expected) => {
  expect(lineDiff(before as string, after as string)).toEqual(expected);
});

it("handles large changed inputs without exceeding the argument limit", () => {
  const before = "a\n" + "\n".repeat(65530) + "b";
  const after = "c\n" + "\n".repeat(65530) + "d";
  let changes: ReturnType<typeof lineDiff> = [];
  expect(() => { changes = lineDiff(before, after); }).not.toThrow();
  expect(changes).toHaveLength(131064);
  expect(changes.filter(change => change.kind === "same")).toHaveLength(0);
  const removed = changes.filter(change => change.kind === "removed");
  const added = changes.filter(change => change.kind === "added");
  expect(removed).toHaveLength(65532);
  expect(added).toHaveLength(65532);
  expect(removed.map(change => change.text).join("\n")).toBe(before);
  expect(added.map(change => change.text).join("\n")).toBe(after);
});

it.each(["prefix", "suffix"])("preserves a large shared %s", edge => {
  const shared = "\n".repeat(131064);
  const before = edge === "prefix" ? shared + "Old" : "Old" + shared;
  const after = edge === "prefix" ? shared + "New" : "New" + shared;
  const changes = lineDiff(before, after);
  expect(changes.filter(change => change.kind === "same")).toHaveLength(131064);
  expect(changes.filter(change => change.kind === "removed")).toEqual([{ kind: "removed", text: "Old" }]);
  expect(changes.filter(change => change.kind === "added")).toEqual([{ kind: "added", text: "New" }]);
});
