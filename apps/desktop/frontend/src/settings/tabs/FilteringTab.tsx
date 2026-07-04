import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Select } from "@/components/ui/select";
import type { OrderMode, ScreenScope, SpaceScope, WindowVisibility } from "../../lib/types";
import { ROW, type TabContext } from "../shared";

export function FilteringTab({ ctx }: { ctx: TabContext }) {
  const { settings, t, patch, patchFilters } = ctx;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("Ordering")}</CardTitle>
        </CardHeader>
        <CardContent>
          <label className={ROW}>
            <span>{t("Display order")}</span>
            <Select
              aria-label="Display order"
              value={settings.order}
              onChange={(e) => patch({ order: e.target.value as OrderMode })}
            >
              <option value="recent">{t("Recently focused")}</option>
              <option value="recentlyCreated">{t("Recently created")}</option>
              <option value="alphabetical">{t("Alphabetical")}</option>
              <option value="space">{t("By space")}</option>
            </Select>
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Which windows to show")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Spaces")}</span>
            <Select
              aria-label="Spaces"
              value={settings.filters.spaces}
              onChange={(e) => patchFilters({ spaces: e.target.value as SpaceScope })}
            >
              <option value="all">{t("All Spaces")}</option>
              <option value="active">{t("Active Space only")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Screens")}</span>
            <Select
              aria-label="Screens"
              value={settings.filters.screens}
              onChange={(e) => patchFilters({ screens: e.target.value as ScreenScope })}
            >
              <option value="all">{t("All screens")}</option>
              <option value="active">{t("Active screen only")}</option>
              <option value="cursor">{t("Screen under cursor")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Minimized windows")}</span>
            <Select
              aria-label="Show minimized windows"
              value={settings.filters.showMinimized}
              onChange={(e) => patchFilters({ showMinimized: e.target.value as WindowVisibility })}
            >
              <option value="show">{t("Show")}</option>
              <option value="hide">{t("Hide")}</option>
              <option value="showAtEnd">{t("Show at the end")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Windows of hidden apps")}</span>
            <Select
              aria-label="Show hidden windows"
              value={settings.filters.showHiddenApps}
              onChange={(e) => patchFilters({ showHiddenApps: e.target.value as WindowVisibility })}
            >
              <option value="show">{t("Show")}</option>
              <option value="hide">{t("Hide")}</option>
              <option value="showAtEnd">{t("Show at the end")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Fullscreen windows")}</span>
            <Select
              aria-label="Show fullscreen windows"
              value={settings.filters.showFullscreen}
              onChange={(e) => patchFilters({ showFullscreen: e.target.value as WindowVisibility })}
            >
              <option value="show">{t("Show")}</option>
              <option value="hide">{t("Hide")}</option>
              <option value="showAtEnd">{t("Show at the end")}</option>
            </Select>
          </label>
          <label className={ROW}>
            <span>{t("Show windows without a title")}</span>
            <Checkbox
              aria-label="Show windows without a title"
              checked={settings.filters.showWindowsWithoutTitle}
              onChange={(e) => patchFilters({ showWindowsWithoutTitle: e.target.checked })}
            />
          </label>
        </CardContent>
      </Card>
    </>
  );
}
