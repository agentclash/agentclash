import type { Metadata } from "next";
import { Instrument_Serif, DM_Sans, IBM_Plex_Mono, Geist } from "next/font/google";
import { Analytics } from "@vercel/analytics/react";
import { AuthKitProvider } from "@workos-inc/authkit-nextjs/components";
import { Toaster } from "@/components/ui/sonner";
import "./globals.css";
import { cn } from "@/lib/utils";

const geist = Geist({subsets:['latin'],variable:'--font-sans'});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  variable: "--font-display",
  weight: "400",
});

const dmSans = DM_Sans({
  subsets: ["latin"],
  variable: "--font-body",
});

const ibmPlexMono = IBM_Plex_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  weight: ["400", "500"],
});

const siteUrl = "https://agentclash.dev";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: "AgentClash",
  description:
    "Opensource head-to-head agent evals. Pit your models against each other on real tasks. Same tools, same constraints, scored live — not benchmarks, not vibes.",
  keywords: [
    "AI agents",
    "LLM benchmarks",
    "AI race",
    "model comparison",
    "agent evaluation",
    "AI competition",
    "head-to-head AI",
    "LLM testing",
  ],
  authors: [{ name: "AgentClash" }],
  creator: "AgentClash",
  openGraph: {
    type: "website",
    locale: "en_US",
    url: siteUrl,
    siteName: "AgentClash",
    title: "AgentClash",
    description:
      "Opensource head-to-head agent evals. Pit your models against each other on real tasks. Same tools, same constraints, scored live — not benchmarks, not vibes.",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "AgentClash — Same challenge. Same tools. Six AI models race head-to-head.",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "AgentClash",
    description:
      "Opensource head-to-head agent evals. Pit your models against each other on real tasks. Same tools, same constraints, scored live — not benchmarks, not vibes.",
    images: ["/twitter-image.png"],
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  },
  icons: {
    icon: [
      { url: "/favicon-16x16.png", sizes: "16x16", type: "image/png" },
      { url: "/favicon-32x32.png", sizes: "32x32", type: "image/png" },
      { url: "/favicon-96x96.png", sizes: "96x96", type: "image/png" },
    ],
    apple: [
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={cn("dark font-sans", geist.variable)}>
      <body
        className={`${instrumentSerif.variable} ${dmSans.variable} ${ibmPlexMono.variable}`}
      >
        <AuthKitProvider>{children}</AuthKitProvider>
        <Toaster position="bottom-right" />
        <Analytics />
      </body>
    </html>
  );
}
