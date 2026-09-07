import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import type { MediaViewState } from "../lib/types";
import "./media.css";

export interface MediaPanelHandlers {
  onAction(
    session: number,
    revision: number,
    kind: string,
    positionMS: number,
  ): Promise<void> | void;
  onPin(session: number, revision: number): Promise<number> | void;
  onClose(session: number, revision: number): Promise<void> | void;
  onSize(session: number, revision: number, width: number, height: number): Promise<void> | void;
  onImport(session: number, revision: number): Promise<void> | void;
  onCancelImport(session: number, acceptedRevision: number): Promise<void> | void;
  onReload(session: number, revision: number): Promise<void> | void;
  onRemove(session: number, revision: number): Promise<void> | void;
  onOffset(session: number, revision: number, offsetMS: number): Promise<void> | void;
}

export function MediaPanel({
  state,
  handlers,
  t = (s) => s,
}: {
  state: MediaViewState;
  handlers: MediaPanelHandlers;
  t?: (s: string) => string;
}) {
  const root = useRef<HTMLElement>(null);
  const scopeKey = `${state.scope.provider}:${state.scope.process.pid}:${state.scope.process.launchID}:${state.scope.generation}:${state.scope.trackEpoch}:${state.scope.trackID}`;
  const interactionKey = `${state.session}:${scopeKey}:${state.interactionEpoch ?? 0}`;
  const canSeek =
    state.sample.status === "ready" &&
    state.sample.capabilities.seek &&
    state.sample.track.durationMS > 0;
  const [seek, setSeek] = useState<number | null>(null);
  const seekScope = useRef("");
  const [readingStart, setReadingStart] = useState<number | null>(null);
  const userScroll = useRef(false);
  const importSequence = useRef(0);
  const [pendingImport, setPendingImport] = useState<{
    operation: number;
    session: number;
    revision: number;
  } | null>(null);
  const active = useRef<HTMLLIElement>(null);
  useEffect(() => {
    setSeek(null);
    seekScope.current = "";
    setReadingStart(null);
  }, [interactionKey]);
  useEffect(() => {
    setPendingImport(null);
  }, [state.session]);
  useEffect(() => {
    setSeek(null);
    seekScope.current = "";
  }, [canSeek, state.sample.track.durationMS]);
  useEffect(() => {
    if (readingStart === null) active.current?.scrollIntoView?.({ block: "center" });
  }, [state.activeCue, readingStart]);
  useLayoutEffect(() => {
    const el = state.pinned ? root.current?.closest<HTMLElement>(".ot-dock-panel") : root.current;
    if (!el) return;
    const report = () => {
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0)
        void handlers.onSize(
          state.session,
          state.revision,
          Math.ceil(r.width),
          Math.ceil(r.height),
        );
    };
    report();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(report);
    observer.observe(el);
    return () => observer.disconnect();
  }, [handlers, state.session, state.revision]);
  const dynamicStart = Math.max(0, Math.max(0, state.activeCue) - 12);
  const cueStart = readingStart ?? dynamicStart;
  const cues = useMemo(() => {
    if (!state.lyrics.cues.length) return [];
    return state.lyrics.cues.slice(cueStart, Math.min(state.lyrics.cues.length, cueStart + 25));
  }, [state.lyrics.cues, cueStart]);
  const act = (kind: string, position = 0) =>
    handlers.onAction(state.session, state.revision, kind, position);
  const commitSeek = () => {
    if (seek !== null && seekScope.current === interactionKey && canSeek) void act("seek", seek);
    setSeek(null);
    seekScope.current = "";
  };
  const startImport = () => {
    if (pendingImport) return;
    const owner = {
      operation: ++importSequence.current,
      session: state.session,
      revision: state.revision,
    };
    setPendingImport(owner);
    void Promise.resolve(handlers.onImport(owner.session, owner.revision)).finally(() =>
      setPendingImport((current) => (current?.operation === owner.operation ? null : current)),
    );
  };
  const provider = state.provider === "spotify" ? "Spotify" : "Apple Music";
  return (
    <section
      ref={root}
      className={`ot-media-panel ot-theme-${state.appearance.theme}${state.pinned ? " is-pinned" : ""}`}
      aria-label={t("Media controls")}
    >
      <header
        className="ot-media-header"
        style={
          state.pinned ? { position: "absolute", top: 0, right: 0, left: 0, height: 32 } : undefined
        }
      >
        <span>{provider}</span>
        <div>
          {state.pinnable ? (
            <button
              type="button"
              aria-label={t("Pin media panel")}
              onClick={() => handlers.onPin(state.session, state.revision)}
            >
              ◇
            </button>
          ) : null}
          {state.pinned ? (
            <button
              className="ot-media-close"
              style={{ position: "absolute", top: 0, right: 0, width: 48, height: 32 }}
              type="button"
              aria-label={t("Close media panel")}
              onClick={() => handlers.onClose(state.session, state.revision)}
            >
              ×
            </button>
          ) : null}
        </div>
      </header>
      <div className="ot-media-track">
        {state.artwork.status === "ready" &&
        state.artwork.image.startsWith("data:image/png;base64,") ? (
          <img src={state.artwork.image} alt="" />
        ) : (
          <div className="ot-media-artwork" aria-hidden="true">
            ♫
          </div>
        )}
        <div>
          <strong>{state.sample.track.title || t("Nothing playing")}</strong>
          <p>{state.sample.track.artist}</p>
          <p>{state.sample.track.album}</p>
          {state.artwork.status !== "ready" && state.artwork.reason ? (
            <small>{state.artwork.reason}</small>
          ) : null}
        </div>
      </div>
      {state.error ? (
        <p role="alert" className="ot-media-error">
          {state.error}
        </p>
      ) : null}
      {state.sample.status !== "ready" ? (
        <p className="ot-media-status">{state.sample.reason || t("Media unavailable")}</p>
      ) : (
        <>
          <div className="ot-media-transport">
            <button
              type="button"
              aria-label={t("Previous")}
              disabled={!state.sample.capabilities.previous}
              onClick={() => act("previous")}
            >
              ‹
            </button>
            {state.sample.playback === "playing" ? (
              <button
                type="button"
                aria-label={t("Pause")}
                disabled={!state.sample.capabilities.pause}
                onClick={() => act("pause")}
              >
                Ⅱ
              </button>
            ) : (
              <button
                type="button"
                aria-label={t("Play")}
                disabled={!state.sample.capabilities.play}
                onClick={() => act("play")}
              >
                ▶
              </button>
            )}
            <button
              type="button"
              aria-label={t("Next")}
              disabled={!state.sample.capabilities.next}
              onClick={() => act("next")}
            >
              ›
            </button>
          </div>
          {canSeek ? (
            <input
              type="range"
              aria-label={t("Playback position")}
              min={0}
              max={state.sample.track.durationMS - 1}
              value={seek ?? Math.min(state.positionMS, state.sample.track.durationMS - 1)}
              onChange={(e) => {
                if (seek === null) seekScope.current = interactionKey;
                setSeek(Number(e.target.value));
              }}
              onPointerUp={commitSeek}
              onPointerCancel={() => {
                setSeek(null);
                seekScope.current = "";
              }}
              onBlur={() => {
                setSeek(null);
                seekScope.current = "";
              }}
              onKeyUp={(event) => {
                if (
                  [
                    "ArrowLeft",
                    "ArrowRight",
                    "ArrowUp",
                    "ArrowDown",
                    "Home",
                    "End",
                    "PageUp",
                    "PageDown",
                  ].includes(event.key) &&
                  seek !== null
                )
                  commitSeek();
              }}
            />
          ) : null}
        </>
      )}
      <div className="ot-media-lyrics-toolbar">
        <span>{t("Synchronized lyrics")}</span>
        {state.lyrics.documentID ? (
          <>
            <button type="button" onClick={() => handlers.onReload(state.session, state.revision)}>
              {t("Reload")}
            </button>
            <button type="button" disabled={pendingImport !== null} onClick={startImport}>
              {t("Replace")}
            </button>
            <button type="button" onClick={() => handlers.onRemove(state.session, state.revision)}>
              {t("Remove")}
            </button>
          </>
        ) : (
          <button type="button" disabled={pendingImport !== null} onClick={startImport}>
            {t("Import .lrc")}
          </button>
        )}
        {pendingImport !== null ? (
          <button
            type="button"
            onClick={() => handlers.onCancelImport(pendingImport.session, pendingImport.revision)}
          >
            {t("Cancel import")}
          </button>
        ) : null}
      </div>
      <p className="ot-media-disclosure">
        {t("Lyrics stay on this Mac. Use a timestamped .lrc file.")}
      </p>
      {state.lyrics.status === "ready" ? (
        <>
          <ul
            className="ot-media-lyrics"
            onWheel={() => {
              userScroll.current = true;
            }}
            onPointerDown={() => {
              userScroll.current = true;
            }}
            onKeyDown={() => {
              userScroll.current = true;
            }}
            onScroll={() => {
              if (userScroll.current) {
                userScroll.current = false;
                setReadingStart((current) => current ?? dynamicStart);
              }
            }}
          >
            {cues.map((cue, i) => {
              const index = cueStart + i;
              const current = index === state.activeCue;
              return (
                <li
                  key={`${cue.atMs}:${index}`}
                  ref={current ? active : undefined}
                  aria-current={current ? "true" : undefined}
                >
                  {cue.text}
                </li>
              );
            })}
          </ul>
          {readingStart !== null ? (
            <button type="button" onClick={() => setReadingStart(null)}>
              {t("Follow")}
            </button>
          ) : null}
          <label className="ot-media-offset">
            {t("Lyrics timing offset (ms)")}
            <input
              aria-label={t("Lyrics timing offset (ms)")}
              type="number"
              min={-30000}
              max={30000}
              value={state.lyrics.offsetMS}
              onChange={(e) =>
                handlers.onOffset(
                  state.session,
                  state.revision,
                  Math.max(-30000, Math.min(30000, Number(e.target.value))),
                )
              }
            />
          </label>
        </>
      ) : (
        <p className="ot-media-status">{state.lyrics.reason || t("No synchronized lyrics")}</p>
      )}
    </section>
  );
}
