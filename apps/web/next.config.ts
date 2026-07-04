import type { NextConfig } from "next";

// Served from the root of the custom domain (option-tab.vozniak.dev), so the
// base path stays empty. NEXT_PUBLIC_BASE_PATH can still override it if the
// site is ever published under a project path again.
const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  basePath: process.env.NEXT_PUBLIC_BASE_PATH ?? "",
};

export default nextConfig;
