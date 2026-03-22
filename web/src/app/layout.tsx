import type { Metadata } from "next";
import { Instrument_Serif, DM_Sans, IBM_Plex_Mono } from "next/font/google";
import { Analytics } from "@vercel/analytics/react";
import "./globals.css";

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
  title: {
    default: "AgentClash — Ship the right agent",
    template: "%s | AgentClash",
  },
  description:
    "Pit AI models against each other on real tasks. Same challenge, same tools, scored live on completion, speed, token efficiency, and tool strategy.",
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
    title: "AgentClash — Ship the right agent",
    description:
      "Same challenge. Same tools. Six AI models race head-to-head. Scored live — not benchmarks, not vibes.",
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
    title: "AgentClash — Ship the right agent",
    description:
      "Pit AI models against each other on real tasks. Same tools, same constraints, scored live.",
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
    <html lang="en">
      <body
        className={`${instrumentSerif.variable} ${dmSans.variable} ${ibmPlexMono.variable}`}
      >
        {children}
        <Analytics />
      </body>
    </html>
  );
}
