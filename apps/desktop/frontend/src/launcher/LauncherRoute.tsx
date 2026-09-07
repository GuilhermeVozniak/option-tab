import { useEffect, useRef, useState } from "react";
import type { LauncherPresentation } from "../lib/types";
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
  subscribe(handler: (state: LauncherPresentation) => void): () => void;
}

export function LauncherRoute({
  session,
  transport,
}: {
  session: number;
  transport: LauncherTransport;
}) {
  const [state, setState] = useState<LauncherPresentation | null>(null);
  const revision = useRef(0);
  const retired = useRef(false);
  useEffect(() => {
    let active = true;
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
      setState(next.visible ? next : null);
    };
    const unsubscribe = transport.subscribe(accept);
    void transport.getState(session).then((next) => next && accept(next));
    return () => {
      active = false;
      unsubscribe();
    };
  }, [session, transport]);
  return state ? (
    <LauncherView presentation={state} onActivate={(...args) => void transport.activate(...args)} />
  ) : null;
}
