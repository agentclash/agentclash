export function json(data: unknown, status = 200) {
  return Response.json(data, { status, headers: { "cache-control": "no-store" } });
}

export class BodyError extends Error {
  constructor(public status: number) { super(status === 413 ? "request_too_large" : "invalid_request"); }
}

export async function readBody(req: Request, limit: number, signal = req.signal): Promise<string> {
  if (Number(req.headers.get("content-length")) > limit) throw new BodyError(413);
  if (!req.body) return "";
  const reader = req.body.getReader();
  const abort = () => { void reader.cancel().catch(() => {}); };
  signal.addEventListener("abort", abort, { once: true });
  const chunks: Uint8Array[] = [];
  let bytes = 0;
  try {
    signal.throwIfAborted();
    for (;;) {
      const { done, value } = await reader.read();
      signal.throwIfAborted();
      if (done) break;
      bytes += value.byteLength;
      if (bytes > limit) { abort(); throw new BodyError(413); }
      chunks.push(value);
    }
    return Buffer.concat(chunks).toString("utf8");
  } finally { signal.removeEventListener("abort", abort); reader.releaseLock(); }
}
