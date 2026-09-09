import { describe, expect, it } from "vitest";
import { detectPlatform, publishedDownloadUrl } from "./download";

describe("detectPlatform", () => {
  it.each([
    ["Mozilla/5.0 (Macintosh; Intel Mac OS X)", "darwin"],
    ["Mozilla/5.0 (Windows NT 10.0)", "windows"],
    ["Mozilla/5.0 (X11; Linux x86_64)", "linux"],
  ] as const)("%s -> %s", (ua, expected) => {
    expect(detectPlatform(ua)).toBe(expected);
  });
});

describe("published assets", () => {
  it("retains the actually published ARM64 download until universal publication", () => {
    expect(publishedDownloadUrl("darwin")).toBe(
      "https://github.com/GuilhermeVozniak/option-tab/releases/download/v0.4.8/option-tab_0.4.8_darwin_arm64.dmg",
    );
  });
});
