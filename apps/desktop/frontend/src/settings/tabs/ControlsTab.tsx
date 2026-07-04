import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Select } from "@/components/ui/select";
import type {
  OrderMode,
  ReleaseAction,
  ScreenScope,
  SpaceScope,
  VisualStyle,
} from "../../lib/types";
import { ShortcutRecorder } from "../ShortcutRecorder";
import { CHECK_LABEL, HINT, ROW, type TabContext } from "../shared";

export function ControlsTab({ ctx }: { ctx: TabContext }) {
  const { settings, t, patch, patchBehavior, patchShortcut } = ctx;

  const addShortcut = () => {
    const used = new Set(settings.shortcuts.map((s) => s.id));
    let id = 1;
    while (id <= 9 && used.has(id)) id++;
    if (id > 9) return;
    patch({
      shortcuts: [
        ...settings.shortcuts,
        { id, chord: "", enabled: true, scope: { appScope: "all" } },
      ],
    });
  };
  const removeShortcut = (id: number) => {
    if (settings.shortcuts.length <= 1) return;
    patch({ shortcuts: settings.shortcuts.filter((s) => s.id !== id) });
  };

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("Shortcuts")}</CardTitle>
          <CardDescription>
            {t("Configure up to 9 independent shortcuts — all free.")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {settings.shortcuts.map((s) => (
            <div
              className="mb-3 rounded-xl border border-white/12 bg-white/5 p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)]"
              key={s.id}
            >
              <div className="flex flex-wrap items-center gap-2">
                <label className={CHECK_LABEL}>
                  <Checkbox
                    aria-label={`Shortcut ${s.id} enabled`}
                    checked={s.enabled}
                    onChange={(e) => patchShortcut(s.id, { enabled: e.target.checked })}
                  />
                  #{s.id}
                </label>
                <ShortcutRecorder
                  aria-label={`Shortcut ${s.id} chord`}
                  value={s.chord}
                  placeholder={t("Press shortcut keys")}
                  onChordChange={(chord) => patchShortcut(s.id, { chord })}
                />
                <Select
                  aria-label={`Shortcut ${s.id} scope`}
                  value={s.scope.appScope}
                  onChange={(e) =>
                    patchShortcut(s.id, {
                      scope: { ...s.scope, appScope: e.target.value as "all" | "activeApp" },
                    })
                  }
                >
                  <option value="all">{t("All windows")}</option>
                  <option value="activeApp">{t("Active app only")}</option>
                </Select>
                <Select
                  aria-label={`Shortcut ${s.id} style`}
                  value={s.styleOverride ?? ""}
                  onChange={(e) =>
                    patchShortcut(s.id, {
                      styleOverride: (e.target.value || undefined) as VisualStyle | undefined,
                    })
                  }
                >
                  <option value="">{t("Default style")}</option>
                  <option value="thumbnails">{t("Thumbnails")}</option>
                  <option value="appIcons">{t("App icons")}</option>
                  <option value="titles">{t("Titles")}</option>
                </Select>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Remove shortcut ${s.id}`}
                  disabled={settings.shortcuts.length <= 1}
                  onClick={() => removeShortcut(s.id)}
                >
                  ✕
                </Button>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2 opacity-90">
                <Select
                  aria-label={`Shortcut ${s.id} when released`}
                  className="h-7 text-xs"
                  value={s.whenReleased ?? "focusSelected"}
                  onChange={(e) =>
                    patchShortcut(s.id, { whenReleased: e.target.value as ReleaseAction })
                  }
                >
                  <option value="focusSelected">{t("On release: focus selected window")}</option>
                  <option value="doNothing">{t("On release: do nothing")}</option>
                </Select>
                <Select
                  aria-label={`Shortcut ${s.id} spaces`}
                  className="h-7 text-xs"
                  value={s.scope.spaces ?? ""}
                  onChange={(e) =>
                    patchShortcut(s.id, {
                      scope: {
                        ...s.scope,
                        spaces: (e.target.value || undefined) as SpaceScope | undefined,
                      },
                    })
                  }
                >
                  <option value="">{t("Spaces: global default")}</option>
                  <option value="all">{t("Spaces: all")}</option>
                  <option value="active">{t("Spaces: active only")}</option>
                </Select>
                <Select
                  aria-label={`Shortcut ${s.id} screens`}
                  className="h-7 text-xs"
                  value={s.scope.screens ?? ""}
                  onChange={(e) =>
                    patchShortcut(s.id, {
                      scope: {
                        ...s.scope,
                        screens: (e.target.value || undefined) as ScreenScope | undefined,
                      },
                    })
                  }
                >
                  <option value="">{t("Screens: global default")}</option>
                  <option value="all">{t("Screens: all")}</option>
                  <option value="active">{t("Screens: active only")}</option>
                  <option value="cursor">{t("Screens: under cursor")}</option>
                </Select>
                <Select
                  aria-label={`Shortcut ${s.id} order`}
                  className="h-7 text-xs"
                  value={s.scope.order ?? ""}
                  onChange={(e) =>
                    patchShortcut(s.id, {
                      scope: {
                        ...s.scope,
                        order: (e.target.value || undefined) as OrderMode | undefined,
                      },
                    })
                  }
                >
                  <option value="">{t("Order: global default")}</option>
                  <option value="recent">{t("Order: recently focused")}</option>
                  <option value="recentlyCreated">{t("Order: recently created")}</option>
                  <option value="alphabetical">{t("Order: alphabetical")}</option>
                  <option value="space">{t("Order: by space")}</option>
                </Select>
              </div>
            </div>
          ))}
          <Button variant="dashed" disabled={settings.shortcuts.length >= 9} onClick={addShortcut}>
            {t("+ Add shortcut")}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Activation")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Hold modifier to cycle (release to select)")}</span>
            <Checkbox
              aria-label="Hold modifier to cycle"
              checked={settings.behavior.holdToCycle}
              onChange={(e) => patchBehavior({ holdToCycle: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Navigate with vim keys (h / j / k / l)")}</span>
            <Checkbox
              aria-label="Vim keys"
              checked={settings.behavior.vimKeys}
              onChange={(e) => patchBehavior({ vimKeys: e.target.checked })}
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Also select windows using")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Mouse hover")}</span>
            <Checkbox
              aria-label="Select windows on mouse hover"
              checked={settings.behavior.mouseHoverSelect}
              onChange={(e) => patchBehavior({ mouseHoverSelect: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Cursor follows focus (warp the mouse to the focused window)")}</span>
            <Checkbox
              aria-label="Cursor follows focus"
              checked={settings.behavior.cursorFollowFocus}
              onChange={(e) => patchBehavior({ cursorFollowFocus: e.target.checked })}
            />
          </label>
          <p className={HINT}>
            {t(
              "While the switcher is open, hold the modifier and tap W to close, M to minimize, F for fullscreen, H to hide the app, or Q to quit it.",
            )}
          </p>
        </CardContent>
      </Card>
    </>
  );
}
