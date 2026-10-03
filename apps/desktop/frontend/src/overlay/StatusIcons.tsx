import { Badge } from "@/components/ui/badge";

// StatusIcons renders the small AltTab-style markers for a window's state:
// minimized, its app hidden, fullscreen, or living on another Space.
export function StatusIcons({
  minimized,
  hidden,
  fullscreen,
  otherSpace,
  t = (text) => text,
}: {
  minimized: boolean;
  hidden: boolean;
  fullscreen: boolean;
  otherSpace: boolean;
  t?: (text: string) => string;
}) {
  if (!minimized && !hidden && !fullscreen && !otherSpace) return null;
  return (
    <div className="ot-status">
      {minimized ? (
        <Badge
          variant="unstyled"
          className="ot-status-icon ot-status-min"
          role="img"
          aria-label={t("Minimized")}
          title={t("Minimized")}
        >
          –
        </Badge>
      ) : null}
      {hidden ? (
        <Badge
          variant="unstyled"
          className="ot-status-icon ot-status-hidden"
          role="img"
          aria-label={t("Hidden app")}
          title={t("Hidden app")}
        >
          ⊘
        </Badge>
      ) : null}
      {fullscreen ? (
        <Badge
          variant="unstyled"
          className="ot-status-icon ot-status-fs"
          role="img"
          aria-label={t("Fullscreen")}
          title={t("Fullscreen")}
        >
          ⇱
        </Badge>
      ) : null}
      {otherSpace ? (
        <Badge
          variant="unstyled"
          className="ot-status-icon ot-status-space"
          role="img"
          aria-label={t("On another Space")}
          title={t("On another Space")}
        >
          ⧉
        </Badge>
      ) : null}
    </div>
  );
}
