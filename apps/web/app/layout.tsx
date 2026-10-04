import { PRODUCT } from "@option-tab/shared";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { SiteFooter } from "../components/SiteFooter";
import { SiteHeader } from "../components/SiteHeader";
import "./globals.css";

const title = "Option Tab — window switching for macOS";
const description =
  "A free, open-source macOS window switcher with thumbnail previews, custom shortcuts, Dock tools, widgets, and automation. Learn how to use every feature.";

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
  twitter: { card: "summary", title, description },
};

const themeScript = `(function(){try{var t=localStorage.getItem('option-tab-site-theme');document.documentElement.dataset.theme=t==='light'||t==='dark'?t:'system'}catch(e){}})()`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" data-theme="system" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <a href="#main-content" className="skip-link">
          Skip to content
        </a>
        <SiteHeader />
        {children}
        <SiteFooter />
      </body>
    </html>
  );
}
