import { list, put } from "@vercel/blob";
import { NextResponse } from "next/server";

type WaitlistRecord = {
  email: string;
  createdAt: string;
};

const BLOB_PATH = "waitlist.json";

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

async function readWaitlistRecords(): Promise<WaitlistRecord[]> {
  try {
    const { blobs } = await list({ prefix: BLOB_PATH });
    if (blobs.length === 0) return [];
    const res = await fetch(blobs[0].downloadUrl);
    const parsed = (await res.json()) as unknown;
    return Array.isArray(parsed) ? (parsed as WaitlistRecord[]) : [];
  } catch {
    return [];
  }
}

async function writeWaitlistRecords(records: WaitlistRecord[]) {
  await put(BLOB_PATH, JSON.stringify(records, null, 2), {
    addRandomSuffix: false,
    contentType: "application/json",
  });
}

export async function POST(request: Request) {
  const payload = await request.json().catch(() => null);

  if (!payload || typeof payload !== "object") {
    return NextResponse.json(
      { error: "Invalid request body." },
      { status: 400 },
    );
  }

  const email =
    typeof payload.email === "string"
      ? payload.email.trim().toLowerCase()
      : "";

  if (!isValidEmail(email)) {
    return NextResponse.json(
      { error: "Enter a valid email." },
      { status: 400 },
    );
  }

  const records = await readWaitlistRecords();

  if (records.some((record) => record.email === email)) {
    return NextResponse.json({ ok: true, duplicate: true });
  }

  const nextRecord: WaitlistRecord = {
    email,
    createdAt: new Date().toISOString(),
  };

  await writeWaitlistRecords([...records, nextRecord]);

  return NextResponse.json({ ok: true, duplicate: false });
}
