"use client";

import { type Platform } from "@option-tab/shared";
import { useEffect, useState } from "react";
import { buttonVariants } from "@/components/ui/button";
import { detectPlatform, publishedDownloadUrl } from "../lib/download";

const OS_LABEL: Record<Platform, string> = {
  darwin: "macOS (Apple silicon)",
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
      className={buttonVariants({ variant: "default", size: "lg" })}
      href={publishedDownloadUrl(platform)}
    >
      Download for {OS_LABEL[platform]}
    </a>
  );
}
