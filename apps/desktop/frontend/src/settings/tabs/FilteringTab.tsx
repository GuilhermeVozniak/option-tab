import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Switch } from "@/components/ui/switch";
import type { OrderMode, ScreenScope, SpaceScope, WindowVisibility } from "../../lib/types";
import { ROW, type TabContext } from "../shared";

export function FilteringTab({ ctx }: { ctx: TabContext }) {
  const { settings, t, mode, patchModePreferences, patchFilters } = ctx;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("Ordering")}</CardTitle>
          <CardDescription>
            {t("Applies to the switcher selected above. Shortcuts can override this order.")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Display order")}</span>
            <NativeSelect
              aria-label="Display order"
              value={mode === "apps" ? settings.appSwitcher.order : settings.order}
              onChange={(e) => patchModePreferences({ order: e.target.value as OrderMode })}
            >
              <NativeSelectOption value="recent">{t("Recently focused")}</NativeSelectOption>
              <NativeSelectOption value="recentlyCreated">
                {t("Recently created")}
              </NativeSelectOption>
              <NativeSelectOption value="alphabetical">{t("Alphabetical")}</NativeSelectOption>
              <NativeSelectOption value="space">{t("By space")}</NativeSelectOption>
            </NativeSelect>
          </Label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Which windows to show")}</CardTitle>
          <CardDescription>
            {t(
              "Shared by the window and app switchers. Shortcuts can override Spaces and screens.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Spaces")}</span>
            <NativeSelect
              aria-label="Spaces"
              value={settings.filters.spaces}
              onChange={(e) => patchFilters({ spaces: e.target.value as SpaceScope })}
            >
              <NativeSelectOption value="all">{t("All Spaces")}</NativeSelectOption>
              <NativeSelectOption value="active">{t("Active Space only")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Screens")}</span>
            <NativeSelect
              aria-label="Screens"
              value={settings.filters.screens}
              onChange={(e) => patchFilters({ screens: e.target.value as ScreenScope })}
            >
              <NativeSelectOption value="all">{t("All screens")}</NativeSelectOption>
              <NativeSelectOption value="active">{t("Active screen only")}</NativeSelectOption>
              <NativeSelectOption value="cursor">{t("Screen under cursor")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Minimized windows")}</span>
            <NativeSelect
              aria-label="Show minimized windows"
              value={settings.filters.showMinimized}
              onChange={(e) => patchFilters({ showMinimized: e.target.value as WindowVisibility })}
            >
              <NativeSelectOption value="show">{t("Show")}</NativeSelectOption>
              <NativeSelectOption value="hide">{t("Hide")}</NativeSelectOption>
              <NativeSelectOption value="showAtEnd">{t("Show at the end")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Windows of hidden apps")}</span>
            <NativeSelect
              aria-label="Show hidden windows"
              value={settings.filters.showHiddenApps}
              onChange={(e) => patchFilters({ showHiddenApps: e.target.value as WindowVisibility })}
            >
              <NativeSelectOption value="show">{t("Show")}</NativeSelectOption>
              <NativeSelectOption value="hide">{t("Hide")}</NativeSelectOption>
              <NativeSelectOption value="showAtEnd">{t("Show at the end")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Fullscreen windows")}</span>
            <NativeSelect
              aria-label="Show fullscreen windows"
              value={settings.filters.showFullscreen}
              onChange={(e) => patchFilters({ showFullscreen: e.target.value as WindowVisibility })}
            >
              <NativeSelectOption value="show">{t("Show")}</NativeSelectOption>
              <NativeSelectOption value="hide">{t("Hide")}</NativeSelectOption>
              <NativeSelectOption value="showAtEnd">{t("Show at the end")}</NativeSelectOption>
            </NativeSelect>
          </Label>
          <Label appearance="unstyled" className={ROW}>
            <span>{t("Show windows without a title")}</span>
            <Switch
              aria-label="Show windows without a title"
              checked={settings.filters.showWindowsWithoutTitle}
              onCheckedChange={(checked) => patchFilters({ showWindowsWithoutTitle: checked })}
            />
          </Label>
        </CardContent>
      </Card>
    </>
  );
}
