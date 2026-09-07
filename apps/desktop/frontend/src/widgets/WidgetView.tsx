import { useEffect, useMemo, useRef, useState } from "react";
import type { ActionOptions, RenderNode, WidgetActions, WidgetLease } from "../lib/widget-types";
import "./widgets.css";

export interface WidgetViewProps {
  state: { lease: WidgetLease; status: string; reason?: string; root: RenderNode };
  actions: WidgetActions;
  t?: (text: string) => string;
}

const statusCopy: Record<string, string> = {
  disabled: "Widget disabled",
  grantRequired: "Permission required",
  loading: "Loading widget…",
  unavailable: "Widget unavailable",
  error: "Widget error",
};
const nodeKinds = new Set(["row", "column", "text", "icon", "progress", "sparkline", "button"]);

function lifetimeKey(lease: WidgetLease): string {
  return [
    lease.controllerEpoch,
    lease.displayUUID,
    lease.session,
    lease.profileID,
    lease.instanceID,
    lease.digest,
    lease.admissionEpoch,
  ].join("\u001f");
}

function nodeKey(node: RenderNode): string {
  return node.kind === "button" ? `${node.key}\u001f${node.actionToken ?? ""}` : node.key;
}

function validTree(root: RenderNode): boolean {
  let count = 0;
  const keys = new Set<string>();
  const walk = (node: RenderNode, depth: number): boolean => {
    count++;
    if (
      depth > 8 ||
      count > 128 ||
      !node ||
      !nodeKinds.has(node.kind) ||
      typeof node.key !== "string" ||
      node.key.length < 1 ||
      node.key.length > 128 ||
      keys.has(node.key) ||
      typeof node.status !== "string"
    )
      return false;
    keys.add(node.key);
    const children = node.children ?? [];
    if (children.length > 16) return false;
    if (node.kind === "row" || node.kind === "column")
      return children.length > 0 && children.every((child) => walk(child, depth + 1));
    if (children.length !== 0) return false;
    if (node.kind === "text") return typeof node.text === "string" && node.text.length <= 4096;
    if (node.kind === "icon")
      return (
        typeof node.assetToken === "string" &&
        node.assetToken.length > 0 &&
        node.assetToken.length <= 512
      );
    if (node.kind === "progress")
      return (
        node.progress === undefined ||
        (Number.isFinite(node.progress) && node.progress >= 0 && node.progress <= 1)
      );
    if (node.kind === "sparkline")
      return !!node.history && node.history.length <= 120 && node.history.every(Number.isFinite);
    return (
      node.kind === "button" &&
      typeof node.text === "string" &&
      node.text.length <= 1024 &&
      typeof node.actionToken === "string" &&
      node.actionToken.length > 0 &&
      node.actionToken.length <= 512
    );
  };
  return walk(root, 0);
}

function cleanOptions(value: ActionOptions): ActionOptions | null {
  if (!value || !Array.isArray(value.options) || value.options.length > 64) return null;
  const options = value.options.filter(
    (option) =>
      typeof option.token === "string" &&
      option.token.length > 0 &&
      option.token.length <= 512 &&
      typeof option.label === "string" &&
      option.label.length > 0 &&
      option.label.length <= 256,
  );
  if (options.length !== value.options.length) return null;
  if (!value.range) return { options };
  const { min, max, step } = value.range;
  if (![min, max, step].every(Number.isFinite) || min > max || step <= 0 || max - min > 1e12)
    return null;
  return { options, range: { min, max, step } };
}

function NodeStatus({ status, t }: { status: string; t: (text: string) => string }) {
  if (status === "ready") return null;
  return <span className="ot-widget-status">{t(statusCopy[status] ?? "Widget unavailable")}</span>;
}

function AssetNode({ node, lease, actions, t }: NodeProps) {
  const [source, setSource] = useState("");
  useEffect(() => {
    let current = true;
    setSource("");
    void actions
      .asset(lease, node.assetToken ?? "")
      .then((value) => {
        if (current && /^data:image\/png;base64,[A-Za-z0-9+/]+={0,2}$/.test(value))
          setSource(value);
      })
      .catch(() => {});
    return () => {
      current = false;
    };
  }, [actions, lease, node.assetToken]);
  if (node.status !== "ready") return <NodeStatus status={node.status} t={t} />;
  return source ? (
    <img className="ot-widget-icon" src={source} alt={node.text || t("Widget icon")} />
  ) : (
    <span className="ot-widget-status">{t("Loading icon…")}</span>
  );
}

function ActionNode({ node, lease, actions, t }: NodeProps) {
  const [admitted, setAdmitted] = useState<{
    spec: ActionOptions;
    lease: WidgetLease;
    actionToken: string;
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [value, setValue] = useState(0);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const open = () => {
    if (loading || busy || node.status !== "ready") return;
    setLoading(true);
    setError("");
    const admittedLease = lease;
    const actionToken = node.actionToken ?? "";
    void actions
      .options(admittedLease, actionToken)
      .then((result) => {
        if (!alive.current) return;
        const clean = cleanOptions(result);
        if (!clean) throw new Error(t("Action unavailable"));
        setAdmitted({ spec: clean, lease: admittedLease, actionToken });
        if (clean.range) setValue(clean.range.min);
      })
      .catch((reason) => {
        if (alive.current) setError(String(reason));
      })
      .finally(() => {
        if (alive.current) setLoading(false);
      });
  };
  const perform = (optionToken: string, number: number | null) => {
    if (busy || !admitted) return;
    setBusy(true);
    setError("");
    void actions
      .perform(admitted.lease, admitted.actionToken, optionToken, number)
      .then(() => {
        if (alive.current) setAdmitted(null);
      })
      .catch((reason) => {
        if (alive.current) setError(String(reason));
      })
      .finally(() => {
        if (alive.current) setBusy(false);
      });
  };
  return (
    <div>
      <button type="button" disabled={loading || busy || node.status !== "ready"} onClick={open}>
        {loading ? t("Loading…") : node.text}
      </button>
      <NodeStatus status={node.status} t={t} />
      {admitted ? (
        <div className="ot-widget-action" aria-label={t("Widget action")}>
          {admitted.spec.options.length ? (
            <div className="ot-widget-action-options">
              {admitted.spec.options.map((option) => (
                <button
                  type="button"
                  key={option.token}
                  disabled={busy}
                  onClick={() => perform(option.token, null)}
                >
                  {option.label}
                </button>
              ))}
            </div>
          ) : null}
          {admitted.spec.range ? (
            <>
              <input
                aria-label={t("Action value")}
                type="range"
                min={admitted.spec.range.min}
                max={admitted.spec.range.max}
                step={admitted.spec.range.step}
                value={value}
                disabled={busy}
                onChange={(event) => setValue(Number(event.target.value))}
              />
              <button type="button" disabled={busy} onClick={() => perform("", value)}>
                {t("Apply")}
              </button>
            </>
          ) : null}
          {!admitted.spec.range && admitted.spec.options.length === 0 ? (
            <button type="button" disabled={busy} onClick={() => perform("", null)}>
              {t("Run action")}
            </button>
          ) : null}
        </div>
      ) : null}
      {error ? (
        <p className="ot-widget-error" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}

interface NodeProps {
  node: RenderNode;
  lease: WidgetLease;
  actions: WidgetActions;
  t: (text: string) => string;
}

function TrustedNode(props: NodeProps): React.ReactNode {
  const { node, t } = props;
  if (node.kind === "row" || node.kind === "column")
    return (
      <div className={node.kind === "row" ? "ot-widget-row" : "ot-widget-column"}>
        {node.children?.map((child) => (
          <TrustedNode key={nodeKey(child)} {...props} node={child} />
        ))}
      </div>
    );
  if (node.kind === "text")
    return node.status === "ready" ? (
      <span className="ot-widget-text">{node.text}</span>
    ) : (
      <NodeStatus status={node.status} t={t} />
    );
  if (node.kind === "icon") return <AssetNode {...props} />;
  if (node.kind === "progress")
    return node.status === "ready" && node.progress !== undefined ? (
      <progress
        className="ot-widget-progress"
        max={100}
        value={node.progress * 100}
        aria-valuenow={node.progress * 100}
        aria-label={node.text || t("Progress")}
      />
    ) : (
      <NodeStatus status={node.status} t={t} />
    );
  if (node.kind === "sparkline") {
    if (node.status !== "ready") return <NodeStatus status={node.status} t={t} />;
    const values = node.history ?? [];
    const low = Math.min(...values);
    const high = Math.max(...values);
    const span = high - low;
    const points = values
      .map(
        (point, index) =>
          `${values.length === 1 ? 50 : (index / (values.length - 1)) * 100},${span === 0 ? 15 : 30 - ((point - low) / span) * 30}`,
      )
      .join(" ");
    return (
      <svg
        className="ot-widget-sparkline"
        viewBox="0 0 100 30"
        role="img"
        aria-label={node.text || t("History")}
      >
        <polyline points={points} />
      </svg>
    );
  }
  return <ActionNode {...props} />;
}

export function WidgetView({ state, actions, t = (text) => text }: WidgetViewProps) {
  const scope = lifetimeKey(state.lease);
  const valid = useMemo(() => validTree(state.root), [state.root]);
  if (state.status !== "ready" && state.status !== "partial")
    return (
      <section className="ot-widget" aria-label={t("Widget")}>
        <p className="ot-widget-status" role="status">
          {t(statusCopy[state.status] ?? "Widget unavailable")}
        </p>
        {state.reason ? <p className="ot-widget-status">{t(state.reason)}</p> : null}
      </section>
    );
  if (!valid)
    return (
      <section className="ot-widget" aria-label={t("Widget")}>
        <p className="ot-widget-error" role="alert">
          {t("Widget content unavailable")}
        </p>
      </section>
    );
  return (
    <section className="ot-widget" aria-label={t("Widget")} key={scope}>
      <TrustedNode
        key={nodeKey(state.root)}
        node={state.root}
        lease={state.lease}
        actions={actions}
        t={t}
      />
    </section>
  );
}
