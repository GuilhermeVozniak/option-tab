import { type Platform, downloadUrl, latestReleaseUrl } from "@option-tab/shared";

// Single source of truth for the version the landing page advertises.
// Bump this in lockstep with a desktop release tag.
export const APP_VERSION = "0.1.0";

export function detectPlatform(userAgent: string): Platform {
  const ua = userAgent.toLowerCase();
  if (ua.includes("mac")) return "darwin";
  if (ua.includes("win")) return "windows";
  return "linux";
}

export { downloadUrl, latestReleaseUrl };
export type { Platform };
