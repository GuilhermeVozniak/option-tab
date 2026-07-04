import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { Translate } from "../lib/i18n";
import { PermissionRow } from "./PermissionRow";
import { HINT, type PermissionsControl } from "./shared";

interface OnboardingProps {
  permissions: PermissionsControl;
  t: Translate;
  onFinish: () => void;
}

// Onboarding is the first-run wizard: it walks the user through granting the
// two macOS permissions the switcher needs. The permission state is polled
// live (usePermissions), so grants made in System Settings reflect here
// immediately. Finishing marks behavior.onboarded so it never shows again.
export function Onboarding({ permissions, t, onFinish }: OnboardingProps) {
  const allGranted =
    permissions.state.accessibility === "granted" &&
    permissions.state.screenRecording === "granted";
  return (
    <div className="mx-auto flex min-h-[80vh] max-w-[560px] flex-col justify-center py-10">
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">{t("Welcome to Option Tab")}</CardTitle>
          <CardDescription>
            {t(
              "Hold ⌥ and press ⇥ to switch windows. Two macOS permissions are needed before the switcher can work.",
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <PermissionRow
            label="Accessibility"
            display={t("Accessibility")}
            hint={t(
              "Required for the global shortcut and window actions (focus, close, minimize).",
            )}
            state={permissions.state.accessibility}
            t={t}
            onRequest={() => permissions.onRequest("accessibility")}
            onOpenSettings={() => permissions.onOpenSettings("accessibility")}
          />
          <PermissionRow
            label="Screen Recording"
            display={t("Screen Recording")}
            hint={t(
              "Required for live window thumbnails; without it, app icons are shown instead.",
            )}
            state={permissions.state.screenRecording}
            t={t}
            onRequest={() => permissions.onRequest("screenRecording")}
            onOpenSettings={() => permissions.onOpenSettings("screenRecording")}
          />
          <p className={`${HINT} pt-2`}>
            {t(
              "After granting a permission in System Settings, the status above updates automatically.",
            )}
          </p>
          <div className="flex justify-end pt-4">
            <Button
              variant={allGranted ? "default" : "glass"}
              aria-label="Finish onboarding"
              onClick={onFinish}
            >
              {allGranted ? t("Get started") : t("Skip for now")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
