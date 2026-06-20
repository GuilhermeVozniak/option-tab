import { describe, expect, it } from "vitest";
import { PRODUCT, downloadUrl, latestReleaseUrl, releaseAssetName } from "./index";

describe("releaseAssetName", () => {
  it.each([
    ["darwin", "arm64", "1.2.3", "option-tab_1.2.3_darwin_arm64.dmg"],
    ["windows", "amd64", "1.2.3", "option-tab_1.2.3_windows_amd64.zip"],
    ["linux", "amd64", "1.2.3", "option-tab_1.2.3_linux_amd64.tar.gz"],
  ] as const)("%s/%s -> %s", (platform, arch, version, expected) => {
    expect(releaseAssetName(platform, arch, version)).toBe(expected);
  });
});

describe("downloadUrl", () => {
  it("builds a tagged release asset URL", () => {
    expect(downloadUrl("darwin", "arm64", "1.2.3")).toBe(
      `${PRODUCT.repo}/releases/download/v1.2.3/option-tab_1.2.3_darwin_arm64.dmg`,
    );
  });
});

describe("latestReleaseUrl", () => {
  it("points at the latest release page", () => {
    expect(latestReleaseUrl()).toBe(`${PRODUCT.repo}/releases/latest`);
  });
});
