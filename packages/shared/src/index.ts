export const PRODUCT = {
  name: "option-tab",
  displayName: "Option Tab",
  repo: "https://github.com/gui336699/option-tab",
} as const;

export type Platform = "darwin" | "windows" | "linux";
export type Arch = "amd64" | "arm64";

const EXTENSIONS: Record<Platform, string> = {
  darwin: "dmg",
  windows: "zip",
  linux: "tar.gz",
};

export function releaseAssetName(platform: Platform, arch: Arch, version: string): string {
  return `${PRODUCT.name}_${version}_${platform}_${arch}.${EXTENSIONS[platform]}`;
}

export function downloadUrl(platform: Platform, arch: Arch, version: string): string {
  return `${PRODUCT.repo}/releases/download/v${version}/${releaseAssetName(platform, arch, version)}`;
}

export function latestReleaseUrl(): string {
  return `${PRODUCT.repo}/releases/latest`;
}
