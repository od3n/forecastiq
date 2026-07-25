import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { AppHeader } from "@/components/AppHeader";
import { AttributionFooter } from "@/components/AttributionFooter";

// Two typefaces only (doc 02 §1.2): Inter (body/label) + JetBrains Mono (data).
// self-hosted via next/font with font-display: swap (doc 02 §12.6).
const inter = Inter({ subsets: ["latin"], variable: "--font-sans", display: "swap" });
const jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: "--font-mono", display: "swap" });

export const metadata: Metadata = {
  title: "ForecastIQ",
  description:
    "ForecastIQ measures forecast accuracy. We don't deliver weather forecasts.",
  icons: { icon: "/favicon.ico" },
  viewport: "width=device-width, initial-scale=1",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrainsMono.variable}`}>
      <body>
        <a href="#main" className="skip-link">
          Skip to content
        </a>
        <AppHeader />
        <main id="main" className="page">
          {children}
        </main>
        <AttributionFooter />
      </body>
    </html>
  );
}
