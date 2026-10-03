import { type Arch, downloadUrl, latestReleaseUrl, type Platform } from "@option-tab/shared";

// Single source of truth for the version the landing page advertises.
// Bump this in lockstep with a desktop release tag.
export const APP_VERSION = "0.4.8";

// Published assets, not a prediction of the next release. Update only after publication.
export const PUBLISHED_ARCH: Record<Platform, Arch> = {
  darwin: "arm64",
  windows: "amd64",
  linux: "amd64",
};
export function publishedDownloadUrl(platform: Platform): string {
  return downloadUrl(platform, PUBLISHED_ARCH[platform], APP_VERSION);
}

export function detectPlatform(userAgent: string): Platform {
  const ua = userAgent.toLowerCase();
  if (ua.includes("mac")) return "darwin";
  if (ua.includes("win")) return "windows";
  return "linux";
}

export type { Platform };
export { downloadUrl, latestReleaseUrl };
