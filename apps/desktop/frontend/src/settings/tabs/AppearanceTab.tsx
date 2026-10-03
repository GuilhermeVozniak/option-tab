import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
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
          <CardTitle>{t("Style and theme")}</CardTitle>
          <CardDescription>
            {t(
              variant === "dock"
                ? "Choose how windows are represented in Dock previews."
                : "Choose how windows are represented in this switcher.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <ToggleGroup
            appearance="unstyled"
            type="single"
            value={a.style}
            onValueChange={(style) => {
              if (style) patchModeAppearance({ style: style as VisualStyle });
            }}
            aria-label={aria("Visual style")}
            className="ot-settings-style-options"
          >
            {STYLES.map((style) => (
              <ToggleGroupItem
                appearance="unstyled"
                value={style}
                key={style}
                type="button"
                aria-label={aria(`Visual style ${style}`)}
                className={cn(
                  "flex h-24 flex-1 cursor-pointer flex-col items-center justify-center gap-2.5 rounded-xl border transition-all",
                  a.style === style
                    ? "border-primary/60 bg-primary/15 shadow-[inset_0_1px_0_rgba(255,255,255,0.2),0_10px_28px_-12px_rgba(59,130,246,0.7)]"
                    : "border-white/12 bg-white/5 hover:bg-white/10",
                )}
              >
                {STYLE_PREVIEWS[style]}
                <span className="text-xs font-medium">{t(STYLE_LABEL[style])}</span>
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Layout direction")}</span>
            <NativeSelect
              aria-label={aria("Layout direction")}
              value={a.layoutDirection}
              onChange={(e) =>
                patchModeAppearance({ layoutDirection: e.target.value as LayoutDirection })
              }
            >
              <NativeSelectOption value="horizontal">{t("Horizontal")}</NativeSelectOption>
              <NativeSelectOption value="vertical">{t("Vertical")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <div className={ROW}>
            <span>{t("Size")}</span>
            <ToggleGroup
              appearance="unstyled"
              type="single"
              data-slot="segmented"
              aria-label={aria("Size")}
              value={a.sizePreset}
              onValueChange={(value) => {
                if (!value) return;
                const sizePreset = value as SizePreset;
                patchModeAppearance({
                  sizePreset,
                  thumbnailMaxPx: SIZE_PRESET_PX[sizePreset].thumbnail,
                  iconSizePx: SIZE_PRESET_PX[sizePreset].icon,
                });
              }}
              className="inline-flex gap-0.5 rounded-lg border border-white/12 bg-white/6 p-0.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md"
            >
              {(["small", "medium", "large"] as const).map((value) => (
                <ToggleGroupItem
                  appearance="unstyled"
                  key={value}
                  value={value}
                  aria-label={aria(`Size ${value}`)}
                  className={cn(
                    "cursor-pointer rounded-md px-3 py-1 text-xs font-medium text-foreground/60 transition-colors hover:text-foreground",
                    a.sizePreset === value &&
                      "bg-white/15 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.2)]",
                  )}
                >
                  {t(value === "small" ? "Small" : value === "medium" ? "Medium" : "Large")}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <div className={ROW}>
            <span>{t("Theme")}</span>
            <ToggleGroup
              appearance="unstyled"
              type="single"
              data-slot="segmented"
              aria-label={aria("Theme")}
              value={a.theme}
              onValueChange={(theme) => {
                if (theme) patchModeAppearance({ theme: theme as Theme });
              }}
              className="inline-flex gap-0.5 rounded-lg border border-white/12 bg-white/6 p-0.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md"
            >
              {(["light", "dark", "system"] as const).map((value) => (
                <ToggleGroupItem
                  appearance="unstyled"
                  key={value}
                  value={value}
                  aria-label={aria(`Theme ${value}`)}
                  className={cn(
                    "cursor-pointer rounded-md px-3 py-1 text-xs font-medium text-foreground/60 transition-colors hover:text-foreground",
                    a.theme === value &&
                      "bg-white/15 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.2)]",
                  )}
                >
                  {t(value === "light" ? "Light" : value === "dark" ? "Dark" : "System")}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Preview the selected window")}</span>
            <Switch
              aria-label={aria("Preview selected window")}
              checked={a.previewSelected}
              onCheckedChange={(checked) => patchModeAppearance({ previewSelected: checked })}
            />
          </Label>
          {variant !== "dock" && (
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Show on")}</span>
              <NativeSelect
                aria-label={aria("Overlay placement")}
                value={modePlacement}
                onChange={(e) => patchModePreferences({ placement: e.target.value as Placement })}
              >
                <NativeSelectOption value="cursorScreen">
                  {t("Screen under cursor")}
                </NativeSelectOption>
                <NativeSelectOption value="activeScreen">{t("Active screen")}</NativeSelectOption>
                <NativeSelectOption value="focusedWindowScreen">
                  {t("Screen of focused window")}
                </NativeSelectOption>
              </NativeSelect>
            </Label>
          )}
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Accent color")}</span>
            <Input
              aria-label={aria("Accent color")}
              type="color"
              className="h-8 w-12 cursor-pointer rounded-lg border border-white/15 bg-white/10 p-1 backdrop-blur-md"
              value={a.accentColor}
              onChange={(e) => patchModeAppearance({ accentColor: e.target.value })}
            />
          </Label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Layout and sizing")}</CardTitle>
          <CardDescription>
            {t("Fine-tune the grid and the amount of detail in each item.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Auto-size thumbnails")}</span>
            <Switch
              aria-label={aria("Auto-size thumbnails")}
              checked={a.autoSize}
              onCheckedChange={(checked) => patchModeAppearance({ autoSize: checked })}
            />
          </Label>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Surface and effects")}</CardTitle>
          <CardDescription>
            {t("Adjust transparency, corners, and background blur.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Background opacity")}</span>
            <Slider
              aria-label={aria("Background opacity")}
              min={0}
              max={1}
              step={0.05}
              value={[a.backgroundOpacity]}
              onValueChange={([value]) => patchModeAppearance({ backgroundOpacity: value })}
            />
          </Label>
          <Label appearance="unstyled" className={ROW}>
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
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Background blur")}</span>
            <Switch
              aria-label={aria("Background blur")}
              checked={a.blur}
              onCheckedChange={(checked) => patchModeAppearance({ blur: checked })}
            />
          </Label>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Window information")}</CardTitle>
          <CardDescription>
            {t("Choose the labels, badges, and controls shown on each window.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Window title truncation")}</span>
            <NativeSelect
              aria-label={aria("Window title truncation")}
              value={a.titleTruncation}
              onChange={(e) =>
                patchModeAppearance({ titleTruncation: e.target.value as TruncationMode })
              }
            >
              <NativeSelectOption value="end">{t("End")}</NativeSelectOption>
              <NativeSelectOption value="middle">{t("Middle")}</NativeSelectOption>
              <NativeSelectOption value="start">{t("Start")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show window titles")}</span>
            <Switch
              aria-label={aria("Show window titles")}
              checked={a.showTitle}
              onCheckedChange={(checked) => patchModeAppearance({ showTitle: checked })}
            />
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show app icon badge on thumbnails")}</span>
            <Switch
              aria-label={aria("Show app badge")}
              checked={a.showAppBadge}
              onCheckedChange={(checked) => patchModeAppearance({ showAppBadge: checked })}
            />
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show window controls on hover (colored circles)")}</span>
            <Switch
              aria-label={aria("Show window controls")}
              checked={a.showWindowControls}
              onCheckedChange={(checked) => patchModeAppearance({ showWindowControls: checked })}
            />
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show status icons (minimized / hidden / fullscreen)")}</span>
            <Switch
              aria-label={aria("Show status icons")}
              checked={a.showStatusIcons}
              onCheckedChange={(checked) => patchModeAppearance({ showStatusIcons: checked })}
            />
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show Space number labels")}</span>
            <Switch
              aria-label={aria("Show Space number labels")}
              checked={a.showSpaceNumbers}
              onCheckedChange={(checked) => patchModeAppearance({ showSpaceNumbers: checked })}
            />
          </Label>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t("Motion and timing")}</CardTitle>
          <CardDescription>
            {t(
              variant === "dock"
                ? "Control how Dock window previews animate."
                : "Control when the switcher appears and how previews animate.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          {variant !== "dock" && (
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Fade out animation")}</span>
              <Switch
                aria-label={aria("Fade out animation")}
                checked={a.fadeOutAnimation}
                onCheckedChange={(checked) => patchModeAppearance({ fadeOutAnimation: checked })}
              />
            </Label>
          )}
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Fade in the selected-window preview")}</span>
            <Switch
              aria-label={aria("Preview fade in")}
              checked={a.previewFade}
              onCheckedChange={(checked) => patchModeAppearance({ previewFade: checked })}
            />
          </Label>
          {variant !== "dock" && (
            <Label appearance="unstyled" className={ROW}>
              <span>{t("Apparition delay (ms)")}</span>
              <span className="flex items-center gap-3">
                <Slider
                  aria-label={aria("Apparition delay")}
                  min={0}
                  max={2000}
                  step={50}
                  value={[a.apparitionDelayMs]}
                  onValueChange={([value]) => patchModeAppearance({ apparitionDelayMs: value })}
                />
                <span className="w-14 text-right text-xs text-muted-foreground">
                  {a.apparitionDelayMs} ms
                </span>
              </span>
            </Label>
          )}
        </CardContent>
      </Card>
    </>
  );
}
