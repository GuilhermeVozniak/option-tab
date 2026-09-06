import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Select } from "@/components/ui/select";
import type { DockLockDisplay, DockMonitorLockSettings, DockMonitorLockState } from "../lib/types";
import { HINT, ROW } from "./shared";

export function DockMonitorLock({
  value,
  state,
  displays,
  error,
  pending,
  t,
  onChange,
  onEnable,
  onPlace,
  onCancel,
}: {
  value: DockMonitorLockSettings;
  state?: DockMonitorLockState;
  displays: DockLockDisplay[];
  error?: string;
  pending?: boolean;
  t: (s: string) => string;
  onChange: (v: DockMonitorLockSettings) => void;
  onEnable: () => void;
  onPlace: (session: number, revision: number, generation: number) => void;
  onCancel: () => void;
}) {
  const statusLabel: Record<string, string> = {
    disabled: "Disabled",
    starting: "Starting",
    protected: "Protected",
    bypassed: "Bypassed",
    awaitingPlacement: "Awaiting placement",
    placing: "Placing",
    disconnected: "Disconnected",
    unreachable: "Unreachable",
    unavailable: "Unavailable",
  };
  const disconnected =
    value.target === "display" &&
    value.displayUUID &&
    !displays.some((d) => d.uuid === value.displayUUID);
  const targetMatches = value.target === "main" || state?.targetUUID === value.displayUUID;
  const canPlace =
    !!state &&
    targetMatches &&
    (state.status === "protected" || state.status === "awaitingPlacement");
  return (
    <div className="space-y-2">
      <label className={ROW}>
        <span>{t("Lock Dock to a monitor")}</span>
        <Checkbox
          aria-label="Lock Dock to a monitor"
          checked={value.enabled}
          onChange={(e) => {
            onChange({ ...value, enabled: e.target.checked });
            if (e.target.checked) onEnable();
          }}
        />
      </label>
      <label className={ROW}>
        <span>{t("Target monitor")}</span>
        <Select
          aria-label="Target monitor"
          value={value.target === "main" ? "main" : value.displayUUID}
          onChange={(e) =>
            onChange({
              ...value,
              target: e.target.value === "main" ? "main" : "display",
              displayUUID: e.target.value === "main" ? value.displayUUID : e.target.value,
            })
          }
        >
          <option value="main">{t("Main display")}</option>
          {disconnected ? (
            <option value={value.displayUUID}>
              {t("Disconnected display")} ({value.displayUUID})
            </option>
          ) : null}
          {displays.map((d) => (
            <option value={d.uuid} key={d.uuid}>
              {d.name}
            </option>
          ))}
        </Select>
      </label>
      <label className={ROW}>
        <span>{t("Bypass modifier")}</span>
        <Select
          aria-label="Bypass modifier"
          value={value.bypassModifier}
          onChange={(e) =>
            onChange({
              ...value,
              bypassModifier: e.target.value as DockMonitorLockSettings["bypassModifier"],
            })
          }
        >
          {["option", "control", "command", "shift"].map((m) => (
            <option key={m} value={m}>
              {t(m[0].toUpperCase() + m.slice(1))}
            </option>
          ))}
        </Select>
      </label>
      <p className={HINT}>
        {t("Status")}: {t(statusLabel[state?.status || "disabled"] || state?.status || "Disabled")}
        {state?.reason ? `: ${state.reason}` : ""}
      </p>
      {state?.status === "awaitingPlacement" ? (
        <p className={HINT}>{t("Return the Dock manually or use Move Dock here.")}</p>
      ) : null}
      {error ? (
        <p role="alert" className="text-sm text-red-600 dark:text-red-400">
          {error}
        </p>
      ) : null}
      {!state?.placementAvailable ? (
        <p className={HINT}>
          {t(
            "Move the Dock to the selected display manually. Protection starts after its position is verified.",
          )}
        </p>
      ) : (
        <p className={HINT}>
          {t("Move Dock here temporarily moves the pointer. Physical input cancels placement.")}
        </p>
      )}
      {state?.placementAvailable && state.status === "placing" ? (
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("Cancel placement")}
        </Button>
      ) : state?.placementAvailable ? (
        <Button
          type="button"
          variant="outline"
          disabled={!value.enabled || pending || !canPlace}
          onClick={() => state && onPlace(state.session, state.revision, state.generation)}
        >
          {t(pending ? "Starting placement…" : "Move Dock here")}
        </Button>
      ) : null}
    </div>
  );
}
