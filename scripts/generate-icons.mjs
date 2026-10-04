import { copyFile, readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

// Use the renderer already installed and pinned by Next.js in bun.lock.
const webRequire = createRequire(new URL("../apps/web/package.json", import.meta.url));
const nextRequire = createRequire(webRequire.resolve("next/package.json"));
const sharp = nextRequire("sharp");
const source = new URL("../assets/option-tab.svg", import.meta.url);
const svg = await readFile(source);

// Keep the existing 100 px macOS app-icon inset on a 1024 px transparent canvas.
const tile = await sharp(svg, { density: 2400 }).resize(824, 824).png().toBuffer();
const appIcon = await sharp({
  create: {
    width: 1024,
    height: 1024,
    channels: 4,
    background: { r: 0, g: 0, b: 0, alpha: 0 },
  },
})
  .composite([{ input: tile, left: 100, top: 100 }])
  .png()
  .toBuffer();

await sharp(appIcon).toFile(
  fileURLToPath(new URL("../apps/desktop/build/appicon.png", import.meta.url)),
);
await sharp(appIcon)
  .resize(512, 512)
  .toFile(fileURLToPath(new URL("../apps/web/app/icon.png", import.meta.url)));
await copyFile(source, new URL("../apps/web/public/option-tab.svg", import.meta.url));

console.log("Updated desktop app icon, web favicon, and website brand mark.");
