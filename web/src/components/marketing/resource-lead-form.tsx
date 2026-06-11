"use client";

import { useRouter } from "next/navigation";
import type { FormEvent } from "react";
import { useState } from "react";

type Props = {
  source: string;
  resource: string;
  ctaLabel?: string;
  className?: string;
};

export function ResourceLeadForm({
  source,
  resource,
  ctaLabel = "Download the PDFs",
  className = "",
}: Props) {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);

    try {
      const response = await fetch("/api/waitlist", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email,
          source,
          resource,
          intent: "resource-download",
        }),
      });
      const payload = (await response.json().catch(() => null)) as
        | { error?: string }
        | null;

      if (!response.ok) {
        setError(payload?.error ?? "Something went wrong. Try again.");
        return;
      }

      const params = new URLSearchParams({
        email: email.trim().toLowerCase(),
        source,
      });
      router.push(`/resources/eval-checklist/thank-you?${params.toString()}`);
    } catch {
      setError("Something went wrong. Try again.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form
      onSubmit={submit}
      className={`rounded-lg border border-white/[0.1] bg-white/[0.03] p-4 sm:p-5 ${className}`}
    >
      <label
        htmlFor={`resource-email-${source}`}
        className="sr-only"
      >
        Email address
      </label>
      <div className="flex flex-col gap-3 sm:flex-row">
        <input
          id={`resource-email-${source}`}
          type="email"
          required
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="Work email"
          className="min-h-12 flex-1 rounded-lg border border-white/[0.12] bg-black/30 px-4 text-sm text-white outline-none transition-colors placeholder:text-white/25 focus:border-white/35"
        />
        <button
          type="submit"
          disabled={submitting}
          className="min-h-12 rounded-lg bg-white px-5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90 disabled:cursor-not-allowed disabled:bg-white/60"
        >
          {submitting ? "Sending..." : ctaLabel}
        </button>
      </div>
      {error ? (
        <p className="mt-3 text-sm leading-6 text-red-300">{error}</p>
      ) : (
        <p className="mt-3 text-xs leading-5 text-white/40">
          Immediate access to the checklist, worksheet, scorecard, procurement
          guide, and rollout plan.
        </p>
      )}
    </form>
  );
}
