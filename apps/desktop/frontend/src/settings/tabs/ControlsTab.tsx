import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type {
  OrderMode,
  PointerAction,
  ReleaseAction,
  ScreenScope,
  SpaceScope,
  VisualStyle,
  WindowAction,
} from "../../lib/types";
import { ShortcutRecorder } from "../ShortcutRecorder";
import { CHECK_LABEL, HINT, ROW, type TabContext } from "../shared";

const ACTION_LABEL: Record<WindowAction, string> = {
  close: "Close",
  minimize: "Minimize",
  fullscreen: "Fullscreen",
  hide: "Hide app",
  quit: "Quit app",
  newWindow: "New window",
  forceQuit: "Force quit",
  closeAll: "Close all windows",
  minimizeAll: "Minimize all windows",
};

function ActionBindingRow({
  code,
  action,
  bindings,
  patchBehavior,
  t,
}: {
  code: string;
  action: WindowAction;
  bindings: Record<string, WindowAction>;
  patchBehavior: TabContext["patchBehavior"];
  t: TabContext["t"];
}) {
  const [draft, setDraft] = useState(code);
  useEffect(() => setDraft(code), [code]);
  const valid = /^Key[A-Z]$/.test(draft) && (draft === code || !(draft in bindings));
  const commit = () => {
    if (!valid) {
      setDraft(code);
      return;
    }
    if (draft === code) return;
    const next = { ...bindings };
    delete next[code];
    next[draft] = action;
    patchBehavior({ actionBindings: next });
  };
  return (
    <div className="flex items-center gap-2 py-1">
      <Input
        className="w-24"
        aria-label={`Physical key ${code}`}
        aria-invalid={!valid}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
      />
      <Select
        aria-label={`Action for ${code}`}
        value={action}
        onChange={(e) =>
          patchBehavior({ actionBindings: { ...bindings, [code]: e.target.value as WindowAction } })
        }
      >
        {[
          "close",
          "minimize",
          "fullscreen",
          "hide",
          "quit",
          "newWindow",
          "forceQuit",
          "closeAll",
          "minimizeAll",
        ].map((v) => (
          <option key={v} value={v}>
            {t(ACTION_LABEL[v as WindowAction])}
          </option>
        ))}
      </Select>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`Remove action binding ${code}`}
        onClick={() => {
          const next = { ...bindings };
          delete next[code];
          patchBehavior({ actionBindings: next });
        }}
      >
        ✕
      </Button>
    </div>
  );
}

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
          <CardTitle>{t("Switcher interactions")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Middle click")}</span>
            <Select
              aria-label="Middle click action"
              value={settings.behavior.middleClickAction}
              onChange={(e) =>
                patchBehavior({ middleClickAction: e.target.value as PointerAction })
              }
            >
              <option value="none">{t("None")}</option>
              <option value="close">{t("Close")}</option>
              <option value="minimize">{t("Minimize")}</option>
            </Select>
          </label>
          {(["swipeUpAction", "swipeDownAction"] as const).map((field) => (
            <label className={ROW} key={field}>
              <span>{t(field === "swipeUpAction" ? "Swipe up" : "Swipe down")}</span>
              <Select
                aria-label={field === "swipeUpAction" ? "Swipe up action" : "Swipe down action"}
                value={settings.behavior[field]}
                onChange={(e) => patchBehavior({ [field]: e.target.value as PointerAction })}
              >
                {(["none", "close", "minimize", "fullscreen", "hide", "quit"] as const).map((v) => (
                  <option key={v} value={v}>
                    {t(v === "none" ? "None" : ACTION_LABEL[v])}
                  </option>
                ))}
              </Select>
            </label>
          ))}
          {Object.entries(settings.behavior.actionBindings).map(([code, action]) => (
            <ActionBindingRow
              key={code}
              code={code}
              action={action}
              bindings={settings.behavior.actionBindings}
              patchBehavior={patchBehavior}
              t={t}
            />
          ))}
          <Button
            type="button"
            aria-label="Add action binding"
            variant="dashed"
            disabled={Object.keys(settings.behavior.actionBindings).length >= 26}
            onClick={() => {
              const code = Array.from(
                { length: 26 },
                (_, i) => `Key${String.fromCharCode(65 + i)}`,
              ).find((candidate) => !(candidate in settings.behavior.actionBindings));
              if (code)
                patchBehavior({
                  actionBindings: { ...settings.behavior.actionBindings, [code]: "close" },
                });
            }}
          >
            {t("+ Add action binding")}
          </Button>
          <Button variant="ghost" onClick={() => patchBehavior({ actionBindings: {} })}>
            {t("Disable action keys")}
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
            <span>{t("Navigate with arrow keys")}</span>
            <Checkbox
              aria-label="Arrow keys"
              checked={settings.behavior.arrowKeys}
              onChange={(e) => patchBehavior({ arrowKeys: e.target.checked })}
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
          <label className={ROW}>
            <span>{t("Trackpad haptic feedback when the selection changes")}</span>
            <Checkbox
              aria-label="Haptic feedback"
              checked={settings.behavior.hapticFeedback}
              onChange={(e) => patchBehavior({ hapticFeedback: e.target.checked })}
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
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Shortcuts while the switcher is open")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          {(
            [
              ["Focus selected window", "⏎"],
              ["Select next window", "⇥ / → / ↓"],
              ["Select previous window", "⇧⇥ / ← / ↑"],
              ["Cancel", "esc"],
              ["Close window", "modifier + W"],
              ["Minimize/Deminimize window", "modifier + M"],
              ["Fullscreen/Defullscreen window", "modifier + F"],
              ["Quit app", "modifier + Q"],
              ["Hide/Show app", "modifier + H"],
              ["Search", t("type any text")],
            ] as [string, string][]
          ).map(([label, key]) => (
            <div key={label} className={ROW}>
              <span>{t(label)}</span>
              <span className="rounded-md border border-white/15 bg-white/8 px-2 py-0.5 text-xs text-muted-foreground">
                {key}
              </span>
            </div>
          ))}
          <p className={HINT}>
            {t("The modifier is whichever key your shortcut holds (e.g. ⌥ for Option+Tab).")}
          </p>
        </CardContent>
      </Card>
    </>
  );
}
