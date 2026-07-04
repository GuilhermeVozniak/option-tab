// StatusIcons renders the small AltTab-style markers for a window's state:
// minimized, its app hidden, fullscreen, or living on another Space.
export function StatusIcons({
  minimized,
  hidden,
  fullscreen,
  otherSpace,
}: {
  minimized: boolean;
  hidden: boolean;
  fullscreen: boolean;
  otherSpace: boolean;
}) {
  if (!minimized && !hidden && !fullscreen && !otherSpace) return null;
  return (
    <div className="ot-status">
      {minimized ? (
        <span
          className="ot-status-icon ot-status-min"
          role="img"
          aria-label="Minimized"
          title="Minimized"
        >
          –
        </span>
      ) : null}
      {hidden ? (
        <span
          className="ot-status-icon ot-status-hidden"
          role="img"
          aria-label="Hidden app"
          title="Hidden app"
        >
          ⊘
        </span>
      ) : null}
      {fullscreen ? (
        <span
          className="ot-status-icon ot-status-fs"
          role="img"
          aria-label="Fullscreen"
          title="Fullscreen"
        >
          ⇱
        </span>
      ) : null}
      {otherSpace ? (
        <span
          className="ot-status-icon ot-status-space"
          role="img"
          aria-label="On another Space"
          title="On another Space"
        >
          ⧉
        </span>
      ) : null}
    </div>
  );
}
