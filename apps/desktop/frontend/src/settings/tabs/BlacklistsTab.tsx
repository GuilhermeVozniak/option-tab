import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { BlacklistEntry, BlacklistHide } from "../../lib/types";
import { CHECK_LABEL, HINT, type TabContext } from "../shared";

export function BlacklistsTab({ ctx }: { ctx: TabContext }) {
  const { settings, t, patchFilters } = ctx;

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
        {settings.filters.appBlacklist.length === 0 ? (
          <p className={HINT}>{t("No apps blacklisted.")}</p>
        ) : null}
        {settings.filters.appBlacklist.map((entry, i) => (
          <div
            className="mb-2 flex flex-wrap items-center gap-2 rounded-xl border border-white/12 bg-white/5 p-2.5"
            key={`blacklist-${i}`}
          >
            <Input
              aria-label={`Blacklist entry ${i + 1}`}
              type="text"
              className="w-56"
              value={entry.match}
              placeholder="com.example.App or App Name"
              onChange={(e) => patchEntry(i, { match: e.target.value })}
            />
            <Select
              aria-label={`Blacklist hide ${i + 1}`}
              value={entry.hide}
              onChange={(e) => patchEntry(i, { hide: e.target.value as BlacklistHide })}
            >
              <option value="always">{t("Hide: always")}</option>
              <option value="whenNoWindow">{t("Hide: when no open window")}</option>
            </Select>
            <label className={CHECK_LABEL}>
              <Checkbox
                aria-label={`Blacklist ignore shortcuts ${i + 1}`}
                checked={entry.ignoreShortcuts}
                onChange={(e) => patchEntry(i, { ignoreShortcuts: e.target.checked })}
              />
              {t("Ignore shortcuts when active")}
            </label>
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Remove blacklist entry ${i + 1}`}
              onClick={() => setBlacklist(settings.filters.appBlacklist.filter((_, j) => j !== i))}
            >
              ✕
            </Button>
          </div>
        ))}
        <Button
          variant="dashed"
          onClick={() =>
            setBlacklist([
              ...settings.filters.appBlacklist,
              { match: "", hide: "always", ignoreShortcuts: false },
            ])
          }
        >
          {t("+ Add app")}
        </Button>
      </CardContent>
    </Card>
  );
}
