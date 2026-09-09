import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { DockPanelView } from "../dock/DockPanelView";
import { FolderPanel } from "../dock/FolderPanel";
import { system } from "../lib/bridge";
import {
  type LauncherItemPanelState,
  type LauncherItemPanelTransport,
  launcherItemPanel,
} from "../lib/launcher-item-panel-bridge";

export function LauncherItemPanelRoute({
  session,
  transport = launcherItemPanel,
  t = (text) => text,
}: {
  session: number;
  transport?: LauncherItemPanelTransport;
  t?: (text: string) => string;
}) {
  const [state, setState] = useState<LauncherItemPanelState | null>(null);
  const [error, setError] = useState("");
  const [frames, setFrames] = useState<Record<string, string>>({});
  const root = useRef<HTMLDivElement>(null);
  const current = useRef<LauncherItemPanelState | null>(null);
  const highwater = useRef(0);
  const retired = useRef(false);
  const frameSequence = useRef(0);
  const pendingFrames = useRef<{
    revision: number;
    sequence: number;
    frames: Record<string, string>;
  } | null>(null);
  const ownerSession = useRef(0);
  const lastSize = useRef("");
  useEffect(() => {
    let active = true;
    if (ownerSession.current !== session) {
      ownerSession.current = session;
      highwater.current = 0;
      retired.current = false;
      lastSize.current = "";
    }
    current.current = null;
    setState(null);
    setError("");
    setFrames({});
    frameSequence.current = 0;
    pendingFrames.current = null;
    const accept = (next: LauncherItemPanelState) => {
      if (
        !active ||
        retired.current ||
        next.session !== session ||
        next.revision < highwater.current
      )
        return;
      const previousRevision = current.current?.revision ?? 0;
      highwater.current = next.revision;
      if (!next.open) retired.current = true;
      current.current = next.open ? next : null;
      const snapshotSequence = next.windows?.frameSequence ?? 0;
      if (next.revision > previousRevision || snapshotSequence > frameSequence.current) {
        const allowed = new Set(next.windows?.entries.map((entry) => String(entry.windowId)) ?? []);
        let merged = boundedFrames({}, next.windows?.frames ?? {}, allowed);
        frameSequence.current = snapshotSequence;
        const pending = pendingFrames.current;
        if (pending?.revision === next.revision && pending.sequence > frameSequence.current) {
          merged = boundedFrames(merged, pending.frames, allowed);
          frameSequence.current = pending.sequence;
        }
        if (pending && pending.revision <= next.revision) pendingFrames.current = null;
        setFrames(merged);
      }
      if (!next.open) pendingFrames.current = null;
      setState(current.current);
      setError(friendlyError(next.error ?? "", t));
    };
    const off = transport.subscribe({
      update: accept,
      hide: (value) => {
        if (!active || value.session !== session || value.revision < highwater.current) return;
        highwater.current = value.revision;
        retired.current = true;
        pendingFrames.current = null;
        current.current = null;
        setState(null);
        setError("");
        setFrames({});
      },
      frames: (value) => {
        const admitted = current.current;
        if (
          !active ||
          retired.current ||
          value.session !== session ||
          !Number.isSafeInteger(value.revision) ||
          value.revision <= 0 ||
          !Number.isSafeInteger(value.sequence) ||
          value.sequence <= 0
        )
          return;
        if (!admitted || value.revision > admitted.revision) {
          if (value.revision < highwater.current) return;
          const pending = pendingFrames.current;
          if (
            pending &&
            (value.revision < pending.revision ||
              (value.revision === pending.revision && value.sequence <= pending.sequence))
          )
            return;
          pendingFrames.current = {
            revision: value.revision,
            sequence: value.sequence,
            frames: boundedFrames(
              pending?.revision === value.revision ? pending.frames : {},
              value.frames,
            ),
          };
          return;
        }
        if (
          !admitted.windows ||
          value.revision !== admitted.revision ||
          value.sequence <= frameSequence.current
        )
          return;
        frameSequence.current = value.sequence;
        const allowed = new Set(admitted.windows.entries.map((entry) => String(entry.windowId)));
        setFrames((old) => boundedFrames(old, value.frames, allowed));
      },
    });
    void transport
      .getState(session)
      .then((next) => next && accept(next))
      .catch((reason) => {
        if (active && !retired.current && !current.current)
          setError(friendlyError(String(reason), t));
      });
    return () => {
      active = false;
      off();
    };
  }, [session, transport]);
  useLayoutEffect(() => {
    if (!state || !root.current) return;
    let timer = 0;
    const publish = () => {
      timer = 0;
      const admitted = current.current;
      const node = root.current;
      if (!admitted || !node) return;
      const width = Math.min(960, Math.max(120, Math.ceil(node.scrollWidth)));
      const height = Math.min(720, Math.max(96, Math.ceil(node.scrollHeight)));
      const key = `${admitted.session}:${width}:${height}`;
      if (key === lastSize.current) return;
      lastSize.current = key;
      void transport.size(admitted.session, admitted.revision, width, height).catch(() => {});
    };
    const observer =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(() => {
            clearTimeout(timer);
            timer = window.setTimeout(publish, 40);
          });
    observer?.observe(root.current);
    publish();
    return () => {
      clearTimeout(timer);
      observer?.disconnect();
    };
  }, [state?.session, state?.revision, state?.kind, state?.folder?.view, transport]);
  const request = (admitted: LauncherItemPanelState, action: () => Promise<void>) => {
    setError("");
    void action().catch((reason) => {
      if (current.current === admitted) setError(friendlyError(String(reason), t));
    });
  };
  if (!state) return error && !retired.current ? <p role="alert">{error}</p> : null;
  if (state.kind === "windows" && state.windows) {
    const windows = state.windows;
    return (
      <div className="ot-launcher-child is-windows" ref={root}>
        {windows.entries.some((entry) => entry.windowId === windows.selectedWindowId) ? (
          <button
            type="button"
            onClick={() =>
              request(state, () =>
                transport.windowAction!(
                  state.session,
                  state.revision,
                  "hide",
                  windows.selectedWindowId,
                  false,
                ),
              )
            }
          >
            {t("Hide app")}
          </button>
        ) : null}
        <DockPanelView
          state={{
            ...windows,
            error: error || windows.error,
            session: state.session,
            revision: state.revision,
            contentKind: "windows",
            entries: windows.entries.map((entry) => ({
              ...entry,
              thumbnail: frames[String(entry.windowId)],
            })),
          }}
          item={null}
          title={state.title}
          nativeHeader
          onClose={() => request(state, () => transport.close(state.session, state.revision))}
          handlers={{
            onSelectWindow: (childSession, windowID) =>
              request(state, () => transport.selectWindow!(childSession, state.revision, windowID)),
            onFocusWindow: (childSession, windowID) =>
              request(state, () =>
                transport.windowAction!(childSession, state.revision, "focus", windowID, false),
              ),
            onAction: (childSession, kind, windowID) => {
              if (!["close", "minimize", "hide", "fullscreen"].includes(kind)) return;
              const entry = windows.entries.find((value) => value.windowId === windowID);
              request(state, () =>
                transport.windowAction!(
                  childSession,
                  state.revision,
                  kind,
                  windowID,
                  kind === "fullscreen" ? !entry?.fullscreen : false,
                ),
              );
            },
            onSize: () => {},
          }}
          t={t}
        />
      </div>
    );
  }
  return (
    <div className="ot-launcher-child" ref={root}>
      <header className="ot-launcher-child-header">
        <strong title={state.title}>{state.title}</strong>
        <button
          type="button"
          aria-label={t("Close preview")}
          onClick={() => request(state, () => transport.close(state.session, state.revision))}
        >
          ×
        </button>
      </header>
      {error ? <p role="alert">{error}</p> : null}
      {state.kind === "folder" && state.folder ? (
        <>
          <div className="ot-launcher-child-view" aria-label={t("Folder view")}>
            {(["list", "grid"] as const).map((view) => (
              <button
                type="button"
                key={view}
                aria-pressed={state.folder?.view === view}
                onClick={() =>
                  request(state, () => transport.view(state.session, state.revision, view))
                }
              >
                {t(view === "list" ? "List" : "Grid")}
              </button>
            ))}
          </div>
          <FolderPanel
            session={state.session}
            revision={state.revision}
            folder={state.folder}
            view={state.folder.view}
            handlers={{
              onSort: transport.sort,
              onRequestAccess: () => system.openPreferences(),
              onCancelAccess: () => undefined,
              onOpen: transport.open,
            }}
            accessLabel="Relink in Settings"
            t={t}
          />
        </>
      ) : null}
    </div>
  );
}

function friendlyError(value: string, t: (text: string) => string) {
  if (!value) return "";
  if (value.includes("folderOpenRefused")) return t("The item could not be opened.");
  if (value.includes("launcher:retired") || value.includes("stale"))
    return t("This preview is no longer available.");
  return value.slice(0, 240);
}

// One pending revision and the visible snapshot each hold at most30 PNG frames,
// 512KiB per encoded frame and4MiB total. No remote/file URLs enter this renderer.
function boundedFrames(
  old: Record<string, string>,
  incoming: Record<string, string>,
  allowed?: Set<string>,
): Record<string, string> {
  const result: Record<string, string> = {};
  let bytes = 0;
  for (const [id, url] of Object.entries({ ...old, ...incoming })) {
    if (
      !/^[1-9][0-9]*$/.test(id) ||
      (allowed && !allowed.has(id)) ||
      typeof url !== "string" ||
      url.length > 512 * 1024
    )
      continue;
    if (url !== "" && !/^data:image\/png;base64,[A-Za-z0-9+/]*={0,2}$/.test(url)) continue;
    if (Object.keys(result).length >= 30 || bytes + url.length > 4 * 1024 * 1024) continue;
    result[id] = url;
    bytes += url.length;
  }
  return result;
}
