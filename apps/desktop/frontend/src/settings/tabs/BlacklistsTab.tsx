import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { BlacklistEntry, BlacklistHide } from "../../lib/types";
import { CHECK_LABEL, HINT, type TabContext } from "../shared";

function BlacklistRow({
  entry,
  index,
  t,
  onChange,
  onRemove,
  onAdd,
}: {
  entry: BlacklistEntry;
  index: number;
  t: TabContext["t"];
  onChange: (patch: Partial<BlacklistEntry>) => void;
  onRemove: () => void;
  onAdd?: (entry: BlacklistEntry) => void;
}) {
  const [match, setMatch] = useState(entry.match);
  useEffect(() => setMatch(entry.match), [entry.match]);
  const commit = () => {
    const value = match.trim();
    if (!value) {
      if (!onAdd) setMatch(entry.match);
      return;
    }
    if (onAdd) onAdd({ ...entry, match: value });
    else if (value !== entry.match) onChange({ match: value });
    setMatch(value);
  };
  return (
    <div className="mb-2 flex flex-wrap items-center gap-2 rounded-xl border border-white/12 bg-white/5 p-2.5">
      <Input
        aria-label={`Blacklist entry ${index + 1}`}
        type="text"
        className="w-56"
        value={match}
        placeholder="com.example.App or App Name"
        onChange={(e) => setMatch(e.target.value)}
        onBlur={() => {
          if (!onAdd) commit();
        }}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing || e.keyCode === 229) return;
          if (e.key === "Enter") {
            e.preventDefault();
            commit();
          }
          if (e.key === "Escape") {
            e.preventDefault();
            if (onAdd) onRemove();
            else setMatch(entry.match);
          }
        }}
      />
      <Select
        aria-label={`Blacklist hide ${index + 1}`}
        value={entry.hide}
        onChange={(e) => onChange({ hide: e.target.value as BlacklistHide })}
      >
        <option value="always">{t("Hide: always")}</option>
        <option value="whenNoWindow">{t("Hide: when no open window")}</option>
      </Select>
      <label className={CHECK_LABEL}>
        <Checkbox
          aria-label={`Blacklist ignore shortcuts ${index + 1}`}
          checked={entry.ignoreShortcuts}
          onChange={(e) => onChange({ ignoreShortcuts: e.target.checked })}
        />
        {t("Ignore shortcuts when active")}
      </label>
      {onAdd ? (
        <Button type="button" disabled={!match.trim()} onClick={commit}>
          {t("Save app")}
        </Button>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size={onAdd ? "default" : "icon"}
        aria-label={onAdd ? t("Cancel app") : `Remove blacklist entry ${index + 1}`}
        onClick={onRemove}
      >
        {onAdd ? t("Cancel app") : "✕"}
      </Button>
    </div>
  );
}

export function BlacklistsTab({ ctx }: { ctx: TabContext }) {
  const { settings, t, patchFilters } = ctx;
  const [draft, setDraft] = useState<BlacklistEntry | null>(null);

  const setBlacklist = (list: BlacklistEntry[]) => patchFilters({ appBlacklist: list });
  const patchEntry = (i: number, p: Partial<BlacklistEntry>) =>
    setBlacklist(settings.filters.appBlacklist.map((e, j) => (j === i ? { ...e, ...p } : e)));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("Blacklisted apps")}</CardTitle>
        <CardDescription>
          {t(
            "Windows of these apps are never shown. Enter a bundle id (com.apple.Safari) or app name.",
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {settings.filters.appBlacklist.length === 0 && !draft ? (
          <p className={HINT}>{t("No apps blacklisted.")}</p>
        ) : null}
        {settings.filters.appBlacklist.map((entry, i) => (
          <BlacklistRow
            key={`blacklist-${i}`}
            entry={entry}
            index={i}
            t={t}
            onChange={(patch) => patchEntry(i, patch)}
            onRemove={() => setBlacklist(settings.filters.appBlacklist.filter((_, j) => j !== i))}
          />
        ))}
        {draft ? (
          <BlacklistRow
            key="draft"
            entry={draft}
            index={settings.filters.appBlacklist.length}
            t={t}
            onChange={(patch) => setDraft((current) => current && { ...current, ...patch })}
            onRemove={() => setDraft(null)}
            onAdd={(entry) => {
              setBlacklist([...settings.filters.appBlacklist, entry]);
              setDraft(null);
            }}
          />
        ) : null}
        <Button
          variant="dashed"
          disabled={!!draft}
          onClick={() => setDraft({ match: "", hide: "always", ignoreShortcuts: false })}
        >
          {t("+ Add app")}
        </Button>
      </CardContent>
    </Card>
  );
}
