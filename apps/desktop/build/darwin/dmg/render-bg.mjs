// Renders bg.html to bg@1x.png / bg@2x.png with Playwright (from apps/web's
// node_modules), for `tiffutil -cathidpicheck … -out bg.tiff`. One-time asset
// generation — rerun only when bg.html changes:
//   node render-bg.mjs && tiffutil -cathidpicheck bg@1x.png bg@2x.png -out bg.tiff
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const require = createRequire(join(here, "../../../../web/package.json"));
const { chromium } = require("@playwright/test");

const browser = await chromium.launch();
for (const scale of [1, 2]) {
  const page = await browser.newPage({
    viewport: { width: 660, height: 400 },
    deviceScaleFactor: scale,
  });
  await page.goto(`file://${join(here, "bg.html")}`);
  await page.screenshot({ path: join(here, `bg@${scale}x.png`) });
  await page.close();
}
await browser.close();
console.log("rendered bg@1x.png and bg@2x.png");
