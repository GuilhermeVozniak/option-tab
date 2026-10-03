import { useEffect, useRef, useState } from "react";
import type { LauncherBadgeTransport } from "../lib/launcher-badge-types";
import type {
  LauncherItemPanelState,
  LauncherItemPanelTransport,
} from "../lib/launcher-item-panel-bridge";
import type { LauncherPresentation, LauncherPresentationItem } from "../lib/types";
import type { LauncherWidgetState, WidgetActions } from "../lib/widget-types";
import { LauncherView } from "./LauncherView";
import type { LauncherItemMutation } from "./reorder";
import type { LauncherAutoHideHold } from "./useLauncherAutoHideHold";
import { useLauncherBadges } from "./useLauncherBadges";
import type { LauncherInteractionTransport } from "./useLauncherInteractions";

export interface LauncherTransport {
  autoHideHold?: LauncherAutoHideHold;
  badges?: LauncherBadgeTransport;
  getState(session: number): Promise<LauncherPresentation | null>;
  activate(
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ): Promise<void>;
  relaunch?: LauncherTransport["activate"];
  showPanel?: LauncherTransport["activate"];
  itemPanels?: Pick<LauncherItemPanelTransport, "subscribe">;
  mutate?: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemsRevision: string,
    mutation: LauncherItemMutation,
  ) => Promise<void>;
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
  interactions?: LauncherInteractionTransport;
}

export function LauncherRoute({
  session,
  transport,
  t = (text) => text,
  language,
}: {
  session: number;
  transport: LauncherTransport;
  t?: (text: string) => string;
  language?: string;
}) {
  const [state, setState] = useState<LauncherPresentation | null>(null);
  const badges = useLauncherBadges(state, transport.badges);
  const [widgetState, setWidgetState] = useState<LauncherWidgetState | null>(null);
  const widgetStateRef = useRef<LauncherWidgetState | null>(null);
  const [widgetError, setWidgetError] = useState("");
  const [actionError, setActionError] = useState("");
  const stateRef = useRef<LauncherPresentation | null>(null);
  const actionSequence = useRef(0);
  const itemLifetimes = useRef(new Map<string, ItemLifetime>());
  const errorItem = useRef<ItemLifetime | null>(null);

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
    itemLifetimes.current.clear();
    errorItem.current = null;
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
      const nextItems = new Map<string, ItemLifetime>();
      if (next.visible) {
        const sameParent = previous && parentIdentity(previous) === parentIdentity(next);
        for (const item of next.items.flatMap((item) => [item, ...(item.members ?? [])])) {
          const prior = sameParent ? itemLifetimes.current.get(item.id) : undefined;
          const identity = itemIdentity(item);
          nextItems.set(
            item.id,
            prior?.identity === identity ? prior : { id: item.id, identity, since: next.revision },
          );
        }
      }
      itemLifetimes.current = nextItems;
      if (
        (errorItem.current
          ? nextItems.get(errorItem.current.id) !== errorItem.current
          : !previous || previous.revision !== next.revision || previous.epoch !== next.epoch) ||
        !next.visible
      ) {
        setActionError("");
        errorItem.current = null;
      }
      stateRef.current = next.visible ? next : null;
      setState(stateRef.current);
    };
    const unsubscribe = transport.subscribe(accept);
    let childSession = 0;
    let childRevision = 0;
    let childRetired = false;
    const offPanels = transport.itemPanels?.subscribe({
      update: (next: LauncherItemPanelState) => {
        const parent = stateRef.current;
        const item = itemLifetimes.current.get(next.itemID);
        if (
          !active ||
          retired.current ||
          !parent ||
          !item ||
          next.parentEpoch !== parent.epoch ||
          next.parentSession !== parent.session ||
          next.displayUUID !== parent.displayUUID ||
          next.profileID !== parent.profileID ||
          !next.parentRevision ||
          next.parentRevision < item.since ||
          next.parentRevision > parent.revision ||
          next.session < childSession ||
          (next.session === childSession && (childRetired || next.revision < childRevision))
        )
          return;
        if (next.session > childSession) {
          childSession = next.session;
          childRetired = false;
          setActionError("");
          errorItem.current = item;
        }
        childRevision = next.revision;
        childRetired = !next.open;
        if (next.error === "previewUnavailable") {
          errorItem.current = item;
          setActionError("previewUnavailable");
        }
      },
      hide: (next) => {
        const parent = stateRef.current;
        if (
          !active ||
          retired.current ||
          !parent ||
          next.session < childSession ||
          (next.session === childSession && next.revision < childRevision)
        )
          return;
        if (
          next.session !== childSession &&
          (next.parentEpoch !== parent.epoch ||
            next.parentSession !== parent.session ||
            next.displayUUID !== parent.displayUUID ||
            next.profileID !== parent.profileID)
        )
          return;
        childSession = next.session;
        childRevision = next.revision;
        childRetired = true;
      },
      frames: () => {},
    });
    void transport
      .getState(session)
      .then((next) => next && accept(next))
      .catch((error) => {
        if (active && !retired.current && !stateRef.current) setActionError(actionFailure(error));
      });
    return () => {
      active = false;
      stateRef.current = null;
      itemLifetimes.current.clear();
      unsubscribe();
      offPanels?.();
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
      .catch(() => active && setWidgetError("Widget content unavailable"));
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
    const item = itemLifetimes.current.get(args[4]);
    errorItem.current = item ?? null;
    setActionError("");
    const failed = (error: unknown) => {
      if (
        item &&
        itemLifetimes.current.get(item.id) === item &&
        actionSequence.current === operation
      )
        setActionError(actionFailure(error));
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
        badges={badges}
        presentation={state}
        onAutoHideHold={transport.autoHideHold}
        onActivate={(...args) => performItem(transport.activate, args)}
        onRelaunch={relaunch ? (...args) => performItem(relaunch, args) : undefined}
        onShowPanel={
          transport.showPanel ? (...args) => performItem(transport.showPanel!, args) : undefined
        }
        onMutate={
          transport.mutate
            ? (...args) => {
                const admitted = stateRef.current;
                if (
                  !admitted ||
                  !admitted.runtimeReorder ||
                  args[3] !== admitted.revision ||
                  args[4] !== admitted.itemsRevision
                )
                  return;
                const operation = ++actionSequence.current;
                errorItem.current = null;
                setActionError("");
                void transport.mutate!(...args).catch((error) => {
                  if (stateRef.current === admitted && operation === actionSequence.current)
                    setActionError(
                      String(error).match(/retired|stale|revision/i)
                        ? "retired"
                        : "rearrangeFailed",
                    );
                });
              }
            : undefined
        }
        widgetState={transport.widgets ? widgetState : undefined}
        widgetActions={widgetActions}
        language={language}
        t={t}
        interactionTransport={transport.interactions}
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
            .catch(() => {
              if (widgetStateRef.current === admitted)
                setWidgetError("This widget action is no longer available. Open it again.");
            });
        }}
      />
      {actionError ? (
        <p className="ot-launcher-action-error" role="alert">
          {t(actionErrorText(actionError))}
        </p>
      ) : null}
      {widgetError ? (
        <p className="ot-launcher-action-error" role="alert">
          {t(widgetError)}
        </p>
      ) : null}
    </>
  ) : actionError && !retired.current ? (
    <p className="ot-launcher-action-error" role="alert">
      {t(actionErrorText(actionError))}
    </p>
  ) : null;
}

type ItemLifetime = { id: string; identity: string; since: number };

// Clock/widget updates may advance the presentation revision without replacing
// an item. Keep the first revision of its uninterrupted semantic lifetime so a
// delayed admission cannot adopt a same-ID replacement or a removed/readded item.
function parentIdentity(state: LauncherPresentation) {
  return JSON.stringify([
    state.epoch,
    state.displayUUID,
    state.session,
    state.profileID,
    state.bounds,
    state.itemsRevision,
  ]);
}

function itemIdentity(item: LauncherPresentationItem) {
  return JSON.stringify([
    item.id,
    item.kind,
    item.name,
    item.status,
    item.running,
    item.referenceRevision,
  ]);
}

function actionFailure(error: unknown) {
  const message = String(error);
  if (/busy/i.test(message)) return "busy";
  if (/retired|stale|revision/i.test(message)) return "retired";
  return "unavailable";
}

function actionErrorText(reason: string) {
  if (reason === "busy") return "The launcher is busy. Try again.";
  if (reason === "retired") return "The launcher changed. Try again.";
  if (reason === "rearrangeFailed") return "The item could not be rearranged.";
  if (reason === "previewUnavailable") return "The item could not be opened.";
  return "The launcher action could not be completed. Try again.";
}
