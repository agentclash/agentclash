import type { Metadata, Viewport } from "next";
import {
  Instrument_Serif,
  DM_Sans,
  IBM_Plex_Mono,
  Geist,
  Geist_Mono,
  Fraunces,
} from "next/font/google";
import { Analytics } from "@vercel/analytics/react";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { Toaster } from "@/components/ui/sonner";
import { AppProviders } from "@/app/providers";
import "./globals.css";
// Framer fonts + reset for components exported via unframer (used by the
// landing-page expanded-cards section). Imported once, globally.
import "@/framer/styles.css";
import { cn } from "@/lib/utils";
import { webmasterVerification } from "@/lib/seo";

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

// Race-mode typography — loaded eagerly so the toggle is instant when flipped.
const fraunces = Fraunces({
  subsets: ["latin"],
  variable: "--font-race-display",
  style: ["normal", "italic"],
  axes: ["SOFT", "WONK", "opsz"],
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-race-mono",
});

const siteUrl = "https://www.agentclash.dev";
const siteDescription =
  "Open-source AI agent evaluation platform. Find where your agents break on real tasks, replay every failure, promote regressions, and gate releases on scorecards.";

// Webmaster-verification metadata (Google Search Console + Bing). See
// lib/seo `webmasterVerification` and docs/frontend/seo-verification.md.
const verification = webmasterVerification();

export const viewport: Viewport = {
  themeColor: "#060606",
};

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: "AgentClash - Open-source AI Agent Evaluation Platform",
  description: siteDescription,
  keywords: [
    "AI agent evaluation",
    "agent evaluation platform",
    "open-source agent evals",
    "AI agent regression testing",
    "coding agent benchmark",
    "LLM agent evaluation",
    "agent eval CI",
    "agent failure debugging",
    "agent regression testing",
    "agent evaluation",
    "LLM testing",
  ],
  authors: [{ name: "AgentClash" }],
  creator: "AgentClash",
  openGraph: {
    type: "website",
    locale: "en_US",
    url: siteUrl,
    siteName: "AgentClash",
    title: "AgentClash - Open-source AI agent evaluation platform",
    description: siteDescription,
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "AgentClash — Find where your agent broke. Replay failures and gate releases.",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "AgentClash - AI agent evaluation platform",
    description: siteDescription,
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
  ...(verification ? { verification } : {}),
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
        className={`${instrumentSerif.variable} ${dmSans.variable} ${ibmPlexMono.variable} ${fraunces.variable} ${geistMono.variable}`}
      >
        <AppProviders>{children}</AppProviders>
        <Toaster position="bottom-right" />
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  );
}
