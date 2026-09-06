import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { Select } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { cn } from "@/lib/utils";
import { SIZE_PRESET_PX } from "../../lib/layout";
import type {
  LayoutDirection,
  Placement,
  SizePreset,
  Theme,
  TruncationMode,
  VisualStyle,
} from "../../lib/types";
import { ROW, type TabContext } from "../shared";

// Mini previews for the three visual styles, mirroring AltTab's style picker.
const STYLE_PREVIEWS: Record<VisualStyle, React.ReactNode> = {
  thumbnails: (
    <span className="flex gap-1">
      {[0, 1, 2].map((i) => (
        <span key={i} className="h-8 w-11 rounded-[4px] border border-white/25 bg-white/15" />
      ))}
    </span>
  ),
  appIcons: (
    <span className="flex items-center gap-1.5">
      {[0, 1, 2].map((i) => (
        <span key={i} className="size-7 rounded-lg border border-white/25 bg-white/15" />
      ))}
    </span>
  ),
  titles: (
    <span className="flex w-20 flex-col gap-1.5">
      {[0, 1, 2].map((i) => (
        <span key={i} className="h-2 rounded-full border border-white/20 bg-white/15" />
      ))}
    </span>
  ),
};

const STYLES: VisualStyle[] = ["thumbnails", "appIcons", "titles"];
const STYLE_LABEL: Record<VisualStyle, string> = {
  thumbnails: "Thumbnails",
  appIcons: "App icons",
  titles: "Titles",
};

export function AppearanceTab({
  ctx,
  variant = "switcher",
}: {
  ctx: TabContext;
  variant?: "switcher" | "dock";
}) {
  const aria = (name: string) => {
    if (variant !== "dock") return name;
    const aliases: Record<string, string> = {
      "Preview selected window": "Dock selected preview",
      "Show window controls": "Dock window controls",
    };
    return aliases[name] ?? `Dock ${name[0].toLowerCase()}${name.slice(1)}`;
  };
  const { t, patchModeAppearance, patchModePreferences, modeAppearance: a, modePlacement } = ctx;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("Appearance")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Layout direction")}</span>
            <Select
              aria-label={aria("Layout direction")}
              value={a.layoutDirection}
              onChange={(e) =>
                patchModeAppearance({ layoutDirection: e.target.value as LayoutDirection })
              }
            >
              <option value="horizontal">{t("Horizontal")}</option>
              <option value="vertical">{t("Vertical")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Use titles at window count (0 disables)")}</span>
            <Input
              aria-label={aria("Compact threshold")}
              type="number"
              className="w-24"
              min={0}
              max={1000}
              value={a.compactThreshold}
              onChange={(e) => patchModeAppearance({ compactThreshold: Number(e.target.value) })}
            />
          </label>
          <div className="mb-3 flex gap-3">
            {STYLES.map((style) => (
              <button
                key={style}
                type="button"
                aria-label={aria(`Visual style ${style}`)}
                aria-pressed={a.style === style}
                className={cn(
                  "flex h-24 flex-1 cursor-pointer flex-col items-center justify-center gap-2.5 rounded-xl border transition-all",
                  a.style === style
                    ? "border-primary/60 bg-primary/15 shadow-[inset_0_1px_0_rgba(255,255,255,0.2),0_10px_28px_-12px_rgba(59,130,246,0.7)]"
                    : "border-white/12 bg-white/5 hover:bg-white/10",
                )}
                onClick={() => patchModeAppearance({ style })}
              >
                {STYLE_PREVIEWS[style]}
                <span className="text-xs font-medium">{t(STYLE_LABEL[style])}</span>
              </button>
            ))}
          </div>
          <div className={ROW}>
            <span>{t("Size")}</span>
            <Segmented<SizePreset>
              ariaLabel={aria("Size")}
              value={a.sizePreset}
              options={[
                { value: "small", label: t("Small") },
                { value: "medium", label: t("Medium") },
                { value: "large", label: t("Large") },
              ]}
              onChange={(v) =>
                patchModeAppearance({
                  sizePreset: v,
                  thumbnailMaxPx: SIZE_PRESET_PX[v].thumbnail,
                  iconSizePx: SIZE_PRESET_PX[v].icon,
                })
              }
            />
          </div>
          <div className={ROW}>
            <span>{t("Theme")}</span>
            <Segmented<Theme>
              ariaLabel={aria("Theme")}
              value={a.theme}
              options={[
                { value: "light", label: t("Light") },
                { value: "dark", label: t("Dark") },
                { value: "system", label: t("System") },
              ]}
              onChange={(theme) => patchModeAppearance({ theme })}
            />
          </div>
          <label className={ROW}>
            <span>{t("Preview the selected window")}</span>
            <Checkbox
              aria-label={aria("Preview selected window")}
              checked={a.previewSelected}
              onChange={(e) => patchModeAppearance({ previewSelected: e.target.checked })}
            />
          </label>
          {variant !== "dock" && (
            <label className={ROW}>
              <span>{t("Show on")}</span>
              <Select
                aria-label={aria("Overlay placement")}
                value={modePlacement}
                onChange={(e) => patchModePreferences({ placement: e.target.value as Placement })}
              >
                <option value="cursorScreen">{t("Screen under cursor")}</option>
                <option value="activeScreen">{t("Active screen")}</option>
                <option value="focusedWindowScreen">{t("Screen of focused window")}</option>
              </Select>
            </label>
          )}
          <label className={ROW}>
            <span>{t("Accent color")}</span>
            <input
              aria-label={aria("Accent color")}
              type="color"
              className="h-8 w-12 cursor-pointer rounded-lg border border-white/15 bg-white/10 p-1 backdrop-blur-md"
              value={a.accentColor}
              onChange={(e) => patchModeAppearance({ accentColor: e.target.value })}
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Advanced")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Max columns")}</span>
            <Input
              aria-label={aria("Max columns")}
              type="number"
              className="w-24"
              min={1}
              max={20}
              value={a.maxColumns}
              onChange={(e) => patchModeAppearance({ maxColumns: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Max rows")}</span>
            <Input
              aria-label={aria("Max rows")}
              type="number"
              className="w-24"
              min={1}
              max={20}
              value={a.maxRows}
              onChange={(e) => patchModeAppearance({ maxRows: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Thumbnail size (px)")}</span>
            <Input
              aria-label={aria("Thumbnail size")}
              type="number"
              className="w-24"
              min={64}
              max={1024}
              value={a.thumbnailMaxPx}
              onChange={(e) => patchModeAppearance({ thumbnailMaxPx: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Icon size (px)")}</span>
            <Input
              aria-label={aria("Icon size")}
              type="number"
              className="w-24"
              min={16}
              max={256}
              value={a.iconSizePx}
              onChange={(e) => patchModeAppearance({ iconSizePx: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Title max width (px)")}</span>
            <Input
              aria-label={aria("Title max width")}
              type="number"
              className="w-24"
              min={60}
              max={1000}
              value={a.titleMaxWidthPx}
              onChange={(e) => patchModeAppearance({ titleMaxWidthPx: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Font size (px)")}</span>
            <Input
              aria-label={aria("Font size")}
              type="number"
              className="w-24"
              min={8}
              max={48}
              value={a.fontSizePx}
              onChange={(e) => patchModeAppearance({ fontSizePx: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Background opacity")}</span>
            <Slider
              aria-label={aria("Background opacity")}
              min={0}
              max={1}
              step={0.05}
              value={a.backgroundOpacity}
              onChange={(e) => patchModeAppearance({ backgroundOpacity: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Corner radius (px)")}</span>
            <Input
              aria-label={aria("Corner radius")}
              type="number"
              className="w-24"
              min={0}
              max={64}
              value={a.cornerRadiusPx}
              onChange={(e) => patchModeAppearance({ cornerRadiusPx: Number(e.target.value) })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Background blur")}</span>
            <Checkbox
              aria-label={aria("Background blur")}
              checked={a.blur}
              onChange={(e) => patchModeAppearance({ blur: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Show window titles")}</span>
            <Checkbox
              aria-label={aria("Show window titles")}
              checked={a.showTitle}
              onChange={(e) => patchModeAppearance({ showTitle: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Show app icon badge on thumbnails")}</span>
            <Checkbox
              aria-label={aria("Show app badge")}
              checked={a.showAppBadge}
              onChange={(e) => patchModeAppearance({ showAppBadge: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Auto-size thumbnails")}</span>
            <Checkbox
              aria-label={aria("Auto-size thumbnails")}
              checked={a.autoSize}
              onChange={(e) => patchModeAppearance({ autoSize: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Show window controls on hover (colored circles)")}</span>
            <Checkbox
              aria-label={aria("Show window controls")}
              checked={a.showWindowControls}
              onChange={(e) => patchModeAppearance({ showWindowControls: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Show status icons (minimized / hidden / fullscreen)")}</span>
            <Checkbox
              aria-label={aria("Show status icons")}
              checked={a.showStatusIcons}
              onChange={(e) => patchModeAppearance({ showStatusIcons: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Show Space number labels")}</span>
            <Checkbox
              aria-label={aria("Show Space number labels")}
              checked={a.showSpaceNumbers}
              onChange={(e) => patchModeAppearance({ showSpaceNumbers: e.target.checked })}
            />
          </label>
          {variant !== "dock" && (
            <label className={ROW}>
              <span>{t("Fade out animation")}</span>
              <Checkbox
                aria-label={aria("Fade out animation")}
                checked={a.fadeOutAnimation}
                onChange={(e) => patchModeAppearance({ fadeOutAnimation: e.target.checked })}
              />
            </label>
          )}
          <label className={ROW}>
            <span>{t("Fade in the selected-window preview")}</span>
            <Checkbox
              aria-label={aria("Preview fade in")}
              checked={a.previewFade}
              onChange={(e) => patchModeAppearance({ previewFade: e.target.checked })}
            />
          </label>
          {variant !== "dock" && (
            <label className={ROW}>
              <span>{t("Apparition delay (ms)")}</span>
              <span className="flex items-center gap-3">
                <Slider
                  aria-label={aria("Apparition delay")}
                  min={0}
                  max={2000}
                  step={50}
                  value={a.apparitionDelayMs}
                  onChange={(e) =>
                    patchModeAppearance({ apparitionDelayMs: Number(e.target.value) })
                  }
                />
                <span className="w-14 text-right text-xs text-muted-foreground">
                  {a.apparitionDelayMs} ms
                </span>
              </span>
            </label>
          )}
          <label className={ROW}>
            <span>{t("Window title truncation")}</span>
            <Select
              aria-label={aria("Window title truncation")}
              value={a.titleTruncation}
              onChange={(e) =>
                patchModeAppearance({ titleTruncation: e.target.value as TruncationMode })
              }
            >
              <option value="end">{t("End")}</option>
              <option value="middle">{t("Middle")}</option>
              <option value="start">{t("Start")}</option>
            </Select>
          </label>
        </CardContent>
      </Card>
    </>
  );
}
