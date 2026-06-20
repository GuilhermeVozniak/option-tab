import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

export const metadata: Metadata = {
  title: "Option Tab — the free AltTab for macOS",
  description:
    "A fast, open-source window switcher for macOS, Windows, and Linux. Live thumbnails, fuzzy search, up to 9 shortcuts, and every AltTab Pro feature — 100% free.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
