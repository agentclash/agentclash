import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { NextResponse } from "next/server";

type WaitlistRecord = {
  email: string;
  createdAt: string;
};

const waitlistDirectory = join(process.cwd(), ".data");
const waitlistFilePath = join(waitlistDirectory, "waitlist.json");

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

async function readWaitlistRecords(): Promise<WaitlistRecord[]> {
  try {
    const raw = await readFile(waitlistFilePath, "utf8");
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) ? (parsed as WaitlistRecord[]) : [];
  } catch (error) {
    if (
      error &&
      typeof error === "object" &&
      "code" in error &&
      error.code === "ENOENT"
    ) {
      return [];
    }

    throw error;
  }
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
    typeof payload.email === "string" ? payload.email.trim().toLowerCase() : "";

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

  await mkdir(waitlistDirectory, { recursive: true });
  await writeFile(
    waitlistFilePath,
    JSON.stringify([...records, nextRecord], null, 2),
    "utf8",
  );

  return NextResponse.json({ ok: true, duplicate: false });
}
