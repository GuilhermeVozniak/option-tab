import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { AboutControl, TabContext } from "../shared";
import { ACTIONS_ROW, HINT, PROJECT_URL } from "../shared";

interface AboutTabProps {
  ctx: TabContext;
  about?: AboutControl;
  updateBanner: ReactNode;
  openURL: (url: string) => void;
  checkUpdates: () => void;
}

export function AboutTab({ ctx, about, updateBanner, openURL, checkUpdates }: AboutTabProps) {
  const { t } = ctx;
  return (
    <Card>
      <CardHeader>
        <CardTitle>Option Tab</CardTitle>
        <CardDescription className="font-semibold text-foreground/80">
          {t("Version {v}").replace("{v}", about?.version ?? "dev")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className={HINT}>
          {t(
            "A free, open-source AltTab-style window switcher for macOS. Hold ⌥ and press ⇥ to switch windows.",
          )}
        </p>
        {updateBanner}
        <div className={ACTIONS_ROW}>
          <Button aria-label="Open project website" onClick={() => openURL(PROJECT_URL)}>
            {t("Website / GitHub")}
          </Button>
          <Button aria-label="Send feedback" onClick={() => openURL(`${PROJECT_URL}/issues/new`)}>
            {t("Send feedback…")}
          </Button>
          <Button aria-label="Support this project" onClick={() => openURL(PROJECT_URL)}>
            {t("Support this project ❤️")}
          </Button>
          <Button aria-label="Check for updates" onClick={checkUpdates}>
            {t("Check for updates…")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
