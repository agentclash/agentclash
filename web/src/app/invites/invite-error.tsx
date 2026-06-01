import Link from "next/link";

interface InviteErrorProps {
  title: string;
  message: string;
}

export function InviteError({ title, message }: InviteErrorProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6 text-foreground">
      <div className="w-full max-w-sm rounded-lg border border-border bg-card p-6 text-center">
        <h1 className="text-base font-semibold">{title}</h1>
        <p className="mt-2 text-sm text-muted-foreground">{message}</p>
        <Link
          href="/dashboard"
          className="mt-5 inline-flex h-8 items-center justify-center rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground"
        >
          Go to dashboard
        </Link>
      </div>
    </main>
  );
}
