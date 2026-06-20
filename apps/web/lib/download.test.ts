import { describe, expect, it } from "vitest";
import { detectPlatform } from "./download";

describe("detectPlatform", () => {
  it.each([
    ["Mozilla/5.0 (Macintosh; Intel Mac OS X)", "darwin"],
    ["Mozilla/5.0 (Windows NT 10.0)", "windows"],
    ["Mozilla/5.0 (X11; Linux x86_64)", "linux"],
  ] as const)("%s -> %s", (ua, expected) => {
    expect(detectPlatform(ua)).toBe(expected);
  });
});
