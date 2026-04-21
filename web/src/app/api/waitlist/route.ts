import { get, put } from "@vercel/blob";
import { Resend } from "resend";
import { NextResponse } from "next/server";

type WaitlistRecord = {
  email: string;
  createdAt: string;
};

const BLOB_PATH = "waitlist.json";

function getResend() {
  return new Resend(process.env.RESEND_API_KEY);
}

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

async function readWaitlistRecords(): Promise<WaitlistRecord[]> {
  try {
    const result = await get(BLOB_PATH, { access: "private" });
    if (!result || result.statusCode !== 200) return [];
    const text = await new Response(result.stream).text();
    const parsed = JSON.parse(text) as unknown;
    return Array.isArray(parsed) ? (parsed as WaitlistRecord[]) : [];
  } catch {
    return [];
  }
}

async function writeWaitlistRecords(records: WaitlistRecord[]) {
  await put(BLOB_PATH, JSON.stringify(records, null, 2), {
    access: "private",
    addRandomSuffix: false,
    allowOverwrite: true,
    contentType: "application/json",
  });
}

function buildWelcomeEmail(position: number) {
  return `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="margin:0;padding:0;background:#060606;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#060606;padding:48px 24px">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0">
        <tr><td style="padding-bottom:32px;text-align:center">
          <span style="color:#ffffff;font-size:20px;font-weight:600;letter-spacing:-0.5px">AgentClash</span>
        </td></tr>
        <tr><td style="padding-bottom:24px;text-align:center">
          <span style="color:#ffffff;font-size:32px;font-weight:700;letter-spacing:-1px">You're #${position} in line.</span>
        </td></tr>
        <tr><td style="padding-bottom:32px;text-align:center;color:rgba(255,255,255,0.4);font-size:15px;line-height:1.6">
          Same challenge. Same tools. Six AI models race head-to-head.<br>
          You signed up early — that matters. When we open access,<br>
          early spots go first.
        </td></tr>
        <tr><td style="padding-bottom:32px;text-align:center">
          <table cellpadding="0" cellspacing="0" style="margin:0 auto;border:1px solid rgba(255,255,255,0.08);border-radius:8px;overflow:hidden">
            <tr>
              <td style="padding:16px 24px;text-align:center;border-right:1px solid rgba(255,255,255,0.08)">
                <div style="color:rgba(255,255,255,0.3);font-size:10px;text-transform:uppercase;letter-spacing:1.5px;margin-bottom:4px">Your position</div>
                <div style="color:#ffffff;font-size:28px;font-weight:700">#${position}</div>
              </td>
              <td style="padding:16px 24px;text-align:center">
                <div style="color:rgba(255,255,255,0.3);font-size:10px;text-transform:uppercase;letter-spacing:1.5px;margin-bottom:4px">Status</div>
                <div style="color:#ffffff;font-size:14px;font-weight:500">Early access</div>
              </td>
            </tr>
          </table>
        </td></tr>
        <tr><td style="padding-bottom:24px;text-align:center;color:rgba(255,255,255,0.25);font-size:13px;line-height:1.6">
          <strong style="color:rgba(255,255,255,0.5)">What's coming:</strong><br>
          Head-to-head races &middot; Composite scoring &middot; Full replays<br>
          Failure-to-eval flywheel &middot; Open source
        </td></tr>
        <tr><td style="text-align:center;padding-bottom:32px">
          <a href="https://github.com/agentclash/agentclash" style="display:inline-block;padding:10px 24px;background:rgba(255,255,255,0.9);color:#060606;font-size:13px;font-weight:600;text-decoration:none;border-radius:6px">Star on GitHub</a>
        </td></tr>
        <tr><td style="text-align:center;color:rgba(255,255,255,0.15);font-size:11px;padding-top:24px;border-top:1px solid rgba(255,255,255,0.06)">
          AgentClash &middot; Ship the right agent.
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`;
}

async function sendWelcomeEmail(email: string, position: number) {
  try {
    await getResend().emails.send({
      from: "AgentClash <team@agentclash.dev>",
      to: email,
      subject: `You're #${position} on the AgentClash waitlist`,
      html: buildWelcomeEmail(position),
    });
  } catch (err) {
    console.error("Failed to send welcome email:", err);
  }
}

export async function GET() {
  const records = await readWaitlistRecords();
  return NextResponse.json({ count: records.length });
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
    const position = records.findIndex((r) => r.email === email) + 1;
    return NextResponse.json({
      ok: true,
      duplicate: true,
      position,
      total: records.length,
    });
  }

  const nextRecord: WaitlistRecord = {
    email,
    createdAt: new Date().toISOString(),
  };

  const updated = [...records, nextRecord];
  await writeWaitlistRecords(updated);

  const position = updated.length;

  // Fire and forget — don't block the response
  sendWelcomeEmail(email, position);

  return NextResponse.json({
    ok: true,
    duplicate: false,
    position,
    total: updated.length,
  });
}
