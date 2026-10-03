import { describe, expect, it } from "vitest";
import { downloadUrl, latestReleaseUrl, PRODUCT, releaseAssetName } from "./index";

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

describe("universal macOS release contract", () => {
  it("names the universal asset explicitly", () => {
    expect(releaseAssetName("darwin", "universal", "0.5.0")).toBe(
      "option-tab_0.5.0_darwin_universal.dmg",
    );
  });
  it("rejects a universal asset on a non-macOS platform", () => {
    expect(() => releaseAssetName("linux", "universal", "0.5.0")).toThrow();
  });
});
