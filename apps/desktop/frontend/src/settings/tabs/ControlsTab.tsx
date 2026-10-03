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
  SwitcherMode,
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
        aria-label={t("Physical key {key}").replace("{key}", () => code)}
        aria-invalid={!valid}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") e.currentTarget.blur();
        }}
      />
      <Select
        aria-label={t("Action for {key}").replace("{key}", () => code)}
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
        aria-label={t("Remove action binding {key}").replace("{key}", () => code)}
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

export function ControlsTab({
  ctx,
  switcherGesturesAvailable = false,
}: {
  ctx: TabContext;
  switcherGesturesAvailable?: boolean;
}) {
  const { settings, t, patch, patchModeBehavior: patchBehavior, modeBehavior, patchShortcut } = ctx;

  const addShortcut = () => {
    const used = new Set(settings.shortcuts.map((s) => s.id));
    let id = 1;
    while (id <= 9 && used.has(id)) id++;
    if (id > 9) return;
    patch({
      shortcuts: [
        ...settings.shortcuts,
        { id, chord: "", enabled: true, scope: { appScope: "all" }, mode: "windows" },
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
            {t(
              "Click a shortcut to record new keys. Each shortcut can open a different switcher or set of windows.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {settings.shortcuts.map((s) => (
            <div className="ot-settings-shortcut" key={s.id}>
              <div className="ot-settings-shortcut-main">
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
                <label className="ot-settings-field">
                  <span>{t("Opens")}</span>
                  <Select
                    aria-label={t("Shortcut {id} mode").replace("{id}", String(s.id))}
                    value={s.mode ?? "windows"}
                    onChange={(e) => patchShortcut(s.id, { mode: e.target.value as SwitcherMode })}
                  >
                    <option value="windows">{t("Window switcher")}</option>
                    <option value="apps">{t("App switcher")}</option>
                  </Select>
                </label>
                <label className="ot-settings-field">
                  <span>{t("Include")}</span>
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
                </label>
                <label className="ot-settings-field">
                  <span>{t("Visual style")}</span>
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
                </label>
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
              <details className="ot-settings-shortcut-details">
                <summary>{t("Behavior and overrides")}</summary>
                <p className="ot-settings-hint">
                  {t("Override the shared rules for this shortcut, or keep the defaults.")}
                </p>
                <div className="ot-settings-shortcut-options">
                  <label className="ot-settings-field">
                    <span>{t("On release")}</span>
                    <Select
                      aria-label={`Shortcut ${s.id} when released`}
                      className="h-7 text-xs"
                      value={s.whenReleased ?? "focusSelected"}
                      onChange={(e) =>
                        patchShortcut(s.id, { whenReleased: e.target.value as ReleaseAction })
                      }
                    >
                      <option value="focusSelected">
                        {t("On release: focus selected window")}
                      </option>
                      <option value="doNothing">{t("On release: do nothing")}</option>
                    </Select>
                  </label>
                  <label className="ot-settings-field">
                    <span>{t("Spaces")}</span>
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
                  </label>
                  <label className="ot-settings-field">
                    <span>{t("Screens")}</span>
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
                  </label>
                  <label className="ot-settings-field">
                    <span>{t("Order")}</span>
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
                  </label>
                </div>
              </details>
            </div>
          ))}
          <Button variant="dashed" disabled={settings.shortcuts.length >= 9} onClick={addShortcut}>
            {t("+ Add shortcut")}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Keyboard navigation")}</CardTitle>
          <CardDescription>
            {t("These controls apply while the selected switcher is open.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Hold modifier to cycle (release to select)")}</span>
            <Checkbox
              aria-label="Hold modifier to cycle"
              checked={modeBehavior.holdToCycle}
              onChange={(e) => patchBehavior({ holdToCycle: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Navigate with arrow keys")}</span>
            <Checkbox
              aria-label="Arrow keys"
              checked={modeBehavior.arrowKeys}
              onChange={(e) => patchBehavior({ arrowKeys: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Navigate with vim keys (h / j / k / l)")}</span>
            <Checkbox
              aria-label="Vim keys"
              checked={modeBehavior.vimKeys}
              onChange={(e) => patchBehavior({ vimKeys: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Trackpad haptic feedback when the selection changes")}</span>
            <Checkbox
              aria-label="Haptic feedback"
              checked={modeBehavior.hapticFeedback}
              onChange={(e) => patchBehavior({ hapticFeedback: e.target.checked })}
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Mouse behavior")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Mouse hover")}</span>
            <Checkbox
              aria-label="Select windows on mouse hover"
              checked={modeBehavior.mouseHoverSelect}
              onChange={(e) => patchBehavior({ mouseHoverSelect: e.target.checked })}
            />
          </label>
          <label className={ROW}>
            <span>{t("Cursor follows focus (warp the mouse to the focused window)")}</span>
            <Checkbox
              aria-label="Cursor follows focus"
              checked={modeBehavior.cursorFollowFocus}
              onChange={(e) => patchBehavior({ cursorFollowFocus: e.target.checked })}
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("Window actions")}</CardTitle>
          <CardDescription>
            {t("Act on the selected window using the mouse, gestures, or modifier keys.")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1">
          <label className={ROW}>
            <span>{t("Middle click")}</span>
            <Select
              aria-label={t("Middle click action")}
              value={modeBehavior.middleClickAction}
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
                aria-label={t(field === "swipeUpAction" ? "Swipe up action" : "Swipe down action")}
                disabled={!switcherGesturesAvailable}
                aria-describedby="switcher-swipe-status"
                value={modeBehavior[field]}
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
          <p id="switcher-swipe-status" className={HINT}>
            {t(
              switcherGesturesAvailable
                ? "Precise gesture input is used for switcher swipes. macOS does not reliably expose the number of fingers."
                : "Native switcher gesture input is unavailable.",
            )}
          </p>
          {Object.entries(modeBehavior.actionBindings).map(([code, action]) => (
            <ActionBindingRow
              key={code}
              code={code}
              action={action}
              bindings={modeBehavior.actionBindings}
              patchBehavior={patchBehavior}
              t={t}
            />
          ))}
          <Button
            type="button"
            aria-label={t("Add action binding")}
            variant="dashed"
            disabled={Object.keys(modeBehavior.actionBindings).length >= 26}
            onClick={() => {
              const code = Array.from(
                { length: 26 },
                (_, i) => `Key${String.fromCharCode(65 + i)}`,
              ).find((candidate) => !(candidate in modeBehavior.actionBindings));
              if (code)
                patchBehavior({
                  actionBindings: { ...modeBehavior.actionBindings, [code]: "close" },
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
