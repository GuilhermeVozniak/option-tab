"use client";

import { useEffect, useState } from "react";
import { type Platform, downloadUrl } from "@option-tab/shared";
import { APP_VERSION, detectPlatform } from "../lib/download";

const ARCH: Record<Platform, "amd64" | "arm64"> = {
  darwin: "arm64",
  windows: "amd64",
  linux: "amd64",
};
const OS_LABEL: Record<Platform, string> = {
  darwin: "macOS",
  windows: "Windows",
  linux: "Linux",
};

export function PrimaryDownload() {
  const [platform, setPlatform] = useState<Platform | null>(null);

  useEffect(() => {
    setPlatform(detectPlatform(navigator.userAgent));
  }, []);

  if (!platform) {
    return null;
  }

  return (
    <a
      data-testid="primary-download"
      data-platform={platform}
      href={downloadUrl(platform, ARCH[platform], APP_VERSION)}
    >
      Download for {OS_LABEL[platform]}
    </a>
  );
}
