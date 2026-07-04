import { PRODUCT } from "@option-tab/shared";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import "./globals.css";

const title = "Option Tab — the free AltTab for macOS";
const description =
  "A fast, open-source window switcher for macOS, Windows, and Linux. Live thumbnails, fuzzy search, up to 9 shortcuts, and every AltTab Pro feature — 100% free.";

export const metadata: Metadata = {
  metadataBase: new URL(PRODUCT.site),
  title,
  description,
  alternates: { canonical: "/" },
  openGraph: {
    type: "website",
    url: PRODUCT.site,
    siteName: PRODUCT.displayName,
    title,
    description,
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
