export type Change = { kind: "same" | "removed" | "added"; text: string };
const tokens = (text: string) => text.match(/\s+|[\p{L}\p{N}_]+|[^\s\p{L}\p{N}_]/gu) || [];

// Exact, whitespace-preserving word diff. Bound work for unusually large pasted
// instructions; the fallback still preserves both original strings exactly.
export function instructionDiff(before: string, after: string): Change[] {
  const a = tokens(before), b = tokens(after), out: Change[] = [];
  const push = (kind: Change["kind"], text: string) => {
    if (!text) return;
    if (out.at(-1)?.kind === kind) out[out.length - 1].text += text;
    else out.push({ kind, text });
  };
  let start = 0, end = 0;
  while (start < a.length && start < b.length && a[start] === b[start]) start++;
  while (end < a.length - start && end < b.length - start && a[a.length - end - 1] === b[b.length - end - 1]) end++;
  push("same", a.slice(0, start).join(""));
  const old = a.slice(start, a.length - end), next = b.slice(start, b.length - end);
  if (old.length * next.length > 1_000_000) {
    push("removed", old.join("")); push("added", next.join(""));
  } else {
    const width = next.length + 1, table = new Uint32Array((old.length + 1) * width);
    for (let i = old.length - 1; i >= 0; i--)
      for (let j = next.length - 1; j >= 0; j--)
        table[i * width + j] = old[i] === next[j] ? 1 + table[(i + 1) * width + j + 1] : Math.max(table[(i + 1) * width + j], table[i * width + j + 1]);
    let i = 0, j = 0;
    while (i < old.length || j < next.length) {
      if (i < old.length && j < next.length && old[i] === next[j]) { push("same", old[i++]); j++; }
      else if (j < next.length && (i === old.length || table[i * width + j + 1] > table[(i + 1) * width + j])) push("added", next[j++]);
      else push("removed", old[i++]);
    }
  }
  if (end) push("same", a.slice(a.length - end).join(""));
  return out;
}

export function diffContext(text: string, first: boolean, last: boolean) {
  const parts = tokens(text);
  if (parts.length <= 24) return text;
  return `${first ? "… " : parts.slice(0, 12).join("") + " … "}${last ? "" : parts.slice(-12).join("")}`;
}
