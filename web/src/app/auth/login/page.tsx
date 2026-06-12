import { withAuth } from "@workos-inc/authkit-nextjs";
import Link from "next/link";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { isReturningVisitor } from "@/lib/auth/returning";
import { ClashMark } from "@/components/marketing/clash-mark";
import { LightSpeed } from "./lightspeed";
import { SignInButton } from "./sign-in-button";
import { TiltCard } from "./tilt-card";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ returnTo?: string; mode?: string; error?: string }>;
}) {
  const { returnTo: rawReturnTo, mode: rawMode, error } = await searchParams;
  const returnTo = sanitizeReturnTo(rawReturnTo);
  const { user } = await withAuth();
  if (user) redirect(returnTo);

  // Choose sign-in vs sign-up: an explicit ?mode= wins, then the returning-visitor
  // hint cookie, then default to sign-up for a brand-new visitor.
  const returning = await isReturningVisitor();
  const mode: "signin" | "signup" =
    rawMode === "signin" || rawMode === "signup"
      ? rawMode
      : returning
        ? "signin"
        : "signup";

  const heading = mode === "signup" ? "Create your account" : "Welcome back";
  const subcopy =
    mode === "signup"
      ? "Run your first head-to-head agent race in minutes."
      : "Continue to your AgentClash dashboard.";

  const otherMode = mode === "signup" ? "signin" : "signup";
  const toggleParams = new URLSearchParams({ mode: otherMode });
  if (returnTo !== "/dashboard") toggleParams.set("returnTo", returnTo);
  const toggleHref = `/auth/login?${toggleParams.toString()}`;

  return (
    <main className="relative min-h-screen overflow-hidden bg-[#060606] text-white">
      <div className="absolute inset-0">
        <LightSpeed intensity={1.2} particleCount={24} quality="medium" />
      </div>
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_38%_46%,transparent_0,rgba(0,0,0,0.18)_42%,rgba(0,0,0,0.7)_100%)]" />

      <div className="pointer-events-none relative grid min-h-screen grid-rows-[1fr_auto] lg:grid-cols-[minmax(0,1fr)_minmax(440px,520px)] lg:grid-rows-1">
        <div className="flex flex-col justify-end p-6 sm:p-10 lg:p-14">
          <p className="font-mono text-2xs uppercase tracking-[0.28em] text-white/45">
            Open evals engine
          </p>
          <h1 className="mt-3 max-w-2xl font-mono text-[1.65rem] font-medium uppercase leading-[1.05] tracking-[0.04em] text-white sm:text-4xl lg:text-[2.6rem]">
            Evals for LLMs
            <br />
            and agents.
          </h1>
          <p className="mt-4 max-w-md text-sm leading-6 text-white/55 sm:mt-5 sm:text-sm">
            Run the same task across models. Score on real outcomes. Replay
            every step.
          </p>
        </div>

        <aside className="flex items-center justify-center px-5 py-10 sm:px-8 lg:px-10">
          <div className="pointer-events-auto w-full max-w-[440px] lg:-translate-y-[6vh]">
            <div className="mb-7 flex items-center gap-3">
              <ClashMark className="size-9" />
              <span className="font-mono text-2xs uppercase tracking-[0.26em] text-white/55">
                AgentClash
              </span>
            </div>

            <TiltCard>
              <div className="glass-card glass-shine rounded-2xl p-6 sm:p-8 lg:p-9">
                <h2 className="text-2xl font-semibold text-white">{heading}</h2>
                <p className="mt-2 text-sm leading-6 text-white/60">{subcopy}</p>

                {error === "callback_failed" && (
                  <p className="mt-4 rounded-md border border-red-400/25 bg-red-400/10 px-3 py-2 text-sm text-red-200/90">
                    Something went wrong signing you in. Please try again.
                  </p>
                )}

                <div className="mt-6">
                  <SignInButton mode={mode} returnTo={returnTo} />
                </div>

                <p className="mt-5 text-center text-sm text-white/55">
                  {mode === "signup"
                    ? "Already have an account? "
                    : "New to AgentClash? "}
                  <Link
                    href={toggleHref}
                    className="font-medium text-white/85 underline-offset-4 hover:text-white hover:underline"
                  >
                    {mode === "signup" ? "Sign in" : "Create an account"}
                  </Link>
                </p>
              </div>
            </TiltCard>

            <div className="mt-6 grid gap-2.5 border-l border-white/15 pl-5 text-sm text-white/55">
              <p>
                <span className="text-white/85">Evaluate:</span> same task,
                every model.
              </p>
              <p>
                <span className="text-white/85">Replay:</span> every step
                preserved.
              </p>
              <p>
                <span className="text-white/85">Score:</span> outcomes over
                vibes.
              </p>
            </div>
          </div>
        </aside>
      </div>
    </main>
  );
}
