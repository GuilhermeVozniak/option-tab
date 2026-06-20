import type {
  OrderMode,
  Placement,
  Settings as SettingsModel,
  Theme,
  VisualStyle,
} from "../lib/types";

interface SettingsProps {
  settings: SettingsModel;
  onChange: (next: SettingsModel) => void;
}

// Settings is a controlled preferences form. It never holds state itself: every
// edit produces a new Settings object passed to onChange, so persistence and
// live-apply are the parent's concern (and the form stays trivially testable).
export function Settings({ settings, onChange }: SettingsProps) {
  const patch = (partial: Partial<SettingsModel>) => onChange({ ...settings, ...partial });
  const patchAppearance = (p: Partial<SettingsModel["appearance"]>) =>
    patch({ appearance: { ...settings.appearance, ...p } });
  const patchBehavior = (p: Partial<SettingsModel["behavior"]>) =>
    patch({ behavior: { ...settings.behavior, ...p } });
  const patchShortcut = (id: number, p: Partial<SettingsModel["shortcuts"][number]>) =>
    patch({ shortcuts: settings.shortcuts.map((s) => (s.id === id ? { ...s, ...p } : s)) });

  return (
    <div className="ot-settings">
      <h1>Option Tab — Preferences</h1>

      <section>
        <h2>Appearance</h2>
        <label>
          Visual style
          <select
            aria-label="Visual style"
            value={settings.appearance.style}
            onChange={(e) => patchAppearance({ style: e.target.value as VisualStyle })}
          >
            <option value="thumbnails">Thumbnails</option>
            <option value="appIcons">App icons</option>
            <option value="titles">Titles</option>
          </select>
        </label>
        <label>
          Theme
          <select
            aria-label="Theme"
            value={settings.appearance.theme}
            onChange={(e) => patchAppearance({ theme: e.target.value as Theme })}
          >
            <option value="system">System</option>
            <option value="light">Light</option>
            <option value="dark">Dark</option>
          </select>
        </label>
        <label>
          Accent color
          <input
            aria-label="Accent color"
            type="color"
            value={settings.appearance.accentColor}
            onChange={(e) => patchAppearance({ accentColor: e.target.value })}
          />
        </label>
        <label>
          Max columns
          <input
            aria-label="Max columns"
            type="number"
            min={1}
            max={20}
            value={settings.appearance.maxColumns}
            onChange={(e) => patchAppearance({ maxColumns: Number(e.target.value) })}
          />
        </label>
        <label>
          <input
            aria-label="Auto-size thumbnails"
            type="checkbox"
            checked={settings.appearance.autoSize}
            onChange={(e) => patchAppearance({ autoSize: e.target.checked })}
          />
          Auto-size thumbnails
        </label>
        <label>
          <input
            aria-label="Show window controls"
            type="checkbox"
            checked={settings.appearance.showWindowControls}
            onChange={(e) => patchAppearance({ showWindowControls: e.target.checked })}
          />
          Show window controls on hover
        </label>
      </section>

      <section>
        <h2>Behavior</h2>
        <label>
          Display order
          <select
            aria-label="Display order"
            value={settings.order}
            onChange={(e) => patch({ order: e.target.value as OrderMode })}
          >
            <option value="recent">Recently used</option>
            <option value="alphabetical">Alphabetical</option>
            <option value="space">By space</option>
          </select>
        </label>
        <label>
          Overlay placement
          <select
            aria-label="Overlay placement"
            value={settings.placement}
            onChange={(e) => patch({ placement: e.target.value as Placement })}
          >
            <option value="cursorScreen">Screen under cursor</option>
            <option value="activeScreen">Active screen</option>
            <option value="focusedWindowScreen">Screen of focused window</option>
          </select>
        </label>
        <label>
          <input
            aria-label="Hold modifier to cycle"
            type="checkbox"
            checked={settings.behavior.holdToCycle}
            onChange={(e) => patchBehavior({ holdToCycle: e.target.checked })}
          />
          Hold modifier to cycle (release to select)
        </label>
        <label>
          <input
            aria-label="Start at login"
            type="checkbox"
            checked={settings.behavior.startAtLogin}
            onChange={(e) => patchBehavior({ startAtLogin: e.target.checked })}
          />
          Start at login
        </label>
      </section>

      <section>
        <h2>Shortcuts</h2>
        <p className="ot-hint">Configure up to 9 independent shortcuts — all free.</p>
        {settings.shortcuts.map((s) => (
          <div className="ot-shortcut-row" key={s.id}>
            <label>
              <input
                aria-label={`Shortcut ${s.id} enabled`}
                type="checkbox"
                checked={s.enabled}
                onChange={(e) => patchShortcut(s.id, { enabled: e.target.checked })}
              />
              #{s.id}
            </label>
            <input
              aria-label={`Shortcut ${s.id} chord`}
              type="text"
              value={s.chord}
              onChange={(e) => patchShortcut(s.id, { chord: e.target.value })}
            />
            <select
              aria-label={`Shortcut ${s.id} scope`}
              value={s.scope.appScope}
              onChange={(e) =>
                patchShortcut(s.id, {
                  scope: { ...s.scope, appScope: e.target.value as "all" | "activeApp" },
                })
              }
            >
              <option value="all">All windows</option>
              <option value="activeApp">Active app only</option>
            </select>
          </div>
        ))}
      </section>
    </div>
  );
}
