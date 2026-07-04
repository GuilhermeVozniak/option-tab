import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Translate } from "../lib/i18n";
import type { PermState } from "../lib/types";
import { HINT } from "./shared";

const PERM_LABEL: Record<PermState, string> = {
  granted: "✓ Granted",
  denied: "✕ Denied",
  unknown: "— Not determined",
};

const PERM_BADGE: Record<PermState, "success" | "destructive" | "warning"> = {
  granted: "success",
  denied: "destructive",
  unknown: "warning",
};

interface PermissionRowProps {
  // label is the stable English identifier used in aria-labels; display is the
  // translated, visible name.
  label: string;
  display: string;
  hint: string;
  state: PermState;
  t: Translate;
  onRequest: () => void;
  onOpenSettings: () => void;
}

export function PermissionRow({
  label,
  display,
  hint,
  state,
  t,
  onRequest,
  onOpenSettings,
}: PermissionRowProps) {
  const granted = state === "granted";
  return (
    <div className="flex items-start justify-between gap-4 border-b border-white/8 py-2.5 last:border-b-0">
      <div className="flex flex-col gap-1">
        <span className="flex items-center gap-2.5">
          <Badge variant={PERM_BADGE[state]}>{t(PERM_LABEL[state])}</Badge>
          <span className="text-[13px] font-semibold">{display}</span>
        </span>
        <span className={HINT}>{hint}</span>
      </div>
      {!granted ? (
        <div className="flex shrink-0 gap-2">
          <Button size="sm" aria-label={`Grant ${label}`} onClick={onRequest}>
            {t("Grant…")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            aria-label={`Open ${label} settings`}
            onClick={onOpenSettings}
          >
            {t("Open Settings…")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
