import type { NextConfig } from "next";

// On GitHub Pages the site is served under /option-tab (a project page);
// deploy-web.yml sets NEXT_PUBLIC_BASE_PATH accordingly. Local dev and the
// Playwright e2e serve from the root, so the default stays empty.
const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? "",
};

export default nextConfig;
