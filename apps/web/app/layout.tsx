import type { ReactNode } from "react";

export const metadata = { title: "Option Tab", description: "The option-tab desktop app." };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
