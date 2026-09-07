import { useEffect, useRef, useState } from "react";
import type { LauncherPresentation } from "../lib/types";
import type { LauncherWidgetState, WidgetActions } from "../lib/widget-types";
import { LauncherView } from "./LauncherView";

export interface LauncherTransport {
  getState(session: number): Promise<LauncherPresentation | null>;
  activate(
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ): Promise<void>;
  relaunch?: LauncherTransport["activate"];
  subscribe(handler: (state: LauncherPresentation) => void): () => void;
  widgets?: {
    get(session: number): Promise<LauncherWidgetState>;
    subscribe(handler: (state: LauncherWidgetState) => void): () => void;
    options: WidgetActions["options"];
    perform: WidgetActions["perform"];
    asset: WidgetActions["asset"];
    select(
      epoch: number,
      displayUUID: string,
      session: number,
      profileID: string,
      stackID: string,
      instanceID: string,
    ): Promise<void>;
  };
}

export function LauncherRoute({
  session,
  transport,
  t,
  language,
}: {
  session: number;
  transport: LauncherTransport;
  t?: (text: string) => string;
  language?: string;
}) {
  const [state, setState] = useState<LauncherPresentation | null>(null);
  const [widgetState, setWidgetState] = useState<LauncherWidgetState | null>(null);
  const widgetStateRef = useRef<LauncherWidgetState | null>(null);
  const [widgetError, setWidgetError] = useState("");
  const [actionError, setActionError] = useState("");
  const stateRef = useRef<LauncherPresentation | null>(null);
  const actionSequence = useRef(0);

  const revision = useRef(0);
  const retired = useRef(false);
  const ownerSession = useRef<number | null>(null);
  useEffect(() => {
    let active = true;
    if (ownerSession.current !== session) {
      ownerSession.current = session;
      revision.current = 0;
      retired.current = false;
    }
    stateRef.current = null;
    setState(null);
    setActionError("");
    const accept = (next: LauncherPresentation) => {
      if (
        !active ||
        retired.current ||
        next.session !== session ||
        next.revision < revision.current
      )
        return;
      revision.current = next.revision;
      if (!next.visible) retired.current = true;
      const previous = stateRef.current;
      if (
        !previous ||
        previous.revision !== next.revision ||
        previous.epoch !== next.epoch ||
        !next.visible
      )
        setActionError("");
      stateRef.current = next.visible ? next : null;
      setState(stateRef.current);
    };
    const unsubscribe = transport.subscribe(accept);
    void transport
      .getState(session)
      .then((next) => next && accept(next))
      .catch((error) => {
        if (active && !retired.current && !stateRef.current)
          setActionError(String(error).slice(0, 240));
      });
    return () => {
      active = false;
      stateRef.current = null;
      unsubscribe();
    };
  }, [session, transport]);
  useEffect(() => {
    if (!transport.widgets) return;
    const widgets = transport.widgets;
    widgetStateRef.current = null;
    setWidgetState(null);
    let active = true;
    let widgetRevision = 0;
    let retired = false;
    const accept = (next: LauncherWidgetState) => {
      const parent = state;
      if (
        !active ||
        retired ||
        !parent ||
        next.session !== session ||
        next.revision < widgetRevision ||
        (next.visible &&
          (next.epoch !== parent.epoch ||
            next.displayUUID !== parent.displayUUID ||
            next.profileID !== parent.profileID))
      )
        return;
      widgetRevision = next.revision;
      if (!next.visible) retired = true;
      widgetStateRef.current = next.visible ? next : null;
      setWidgetState(next.visible ? next : null);
      setWidgetError("");
    };
    const unsubscribe = widgets.subscribe(accept);
    void widgets
      .get(session)
      .then(accept)
      .catch((error) => active && setWidgetError(String(error)));
    return () => {
      active = false;
      unsubscribe();
    };
  }, [session, state?.epoch, state?.displayUUID, state?.profileID, transport]);
  const widgetActions: WidgetActions | undefined = transport.widgets
    ? {
        options: transport.widgets.options,
        perform: transport.widgets.perform,
        asset: transport.widgets.asset,
      }
    : undefined;
  const performItem = (
    command: LauncherTransport["activate"],
    args: Parameters<LauncherTransport["activate"]>,
  ) => {
    const admitted = stateRef.current;
    if (!admitted || admitted.session !== session) return;
    const operation = ++actionSequence.current;
    setActionError("");
    const failed = (error: unknown) => {
      if (stateRef.current === admitted && actionSequence.current === operation)
        setActionError(String(error).slice(0, 240));
    };
    try {
      void Promise.resolve(command(...args)).catch(failed);
    } catch (error) {
      failed(error);
    }
  };
  const relaunch = transport.relaunch;
  return state && state.session === session ? (
    <>
      <LauncherView
        presentation={state}
        onActivate={(...args) => performItem(transport.activate, args)}
        onRelaunch={relaunch ? (...args) => performItem(relaunch, args) : undefined}
        widgetState={transport.widgets ? widgetState : undefined}
        widgetActions={widgetActions}
        language={language}
        t={t}
        onSelectWidget={(stackID, instanceID) => {
          if (!widgetState || !transport.widgets) return;
          const widgets = transport.widgets;
          const admitted = widgetState;
          void widgets
            .select(
              admitted.epoch,
              admitted.displayUUID,
              admitted.session,
              admitted.profileID,
              stackID,
              instanceID,
            )
            .catch((error) => {
              if (widgetStateRef.current === admitted) setWidgetError(String(error));
            });
        }}
      />
      {actionError ? (
        <p className="ot-launcher-action-error" role="alert">
          {actionError}
        </p>
      ) : null}
      {widgetError ? (
        <p className="ot-launcher-action-error" role="alert">
          {widgetError}
        </p>
      ) : null}
    </>
  ) : actionError && !retired.current ? (
    <p className="ot-launcher-action-error" role="alert">
      {actionError}
    </p>
  ) : null;
}
