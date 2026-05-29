import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Try CLI — Interactive terminal demos | AgentClash",
  description:
    "Let users try your CLI before they install it. Disposable E2B sandboxes, README badges, zero install.",
  openGraph: {
    title: "AgentClash Try CLI",
    description: "Interactive README demos for developer tools.",
    url: "https://www.agentclash.dev/try",
  },
};

export default function TryCliLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <nav className="border-b border-border px-4 py-3">
        <Link href="/" className="text-sm text-muted-foreground hover:text-foreground">
          ← AgentClash
        </Link>
      </nav>
      {children}
    </div>
  );
}
