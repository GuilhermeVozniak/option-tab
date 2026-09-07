import type { LauncherBadgeEntry } from "../lib/launcher-badge-types";
import type { LauncherMutateCommand } from "../lib/launcher-item-runtime-bridge";
import type { LauncherPresentation, LauncherWidgetNode } from "../lib/types";
import type { LauncherWidgetState, WidgetActions, WidgetLocalized } from "../lib/widget-types";
import { WidgetView } from "../widgets/WidgetView";
import { type LauncherItemCommand, LauncherItemStrip } from "./LauncherItemStrip";
import type { LauncherInteractionTransport } from "./useLauncherInteractions";
import { useLauncherInteractions } from "./useLauncherInteractions";
import "./launcher.css";

function WidgetNode({ node }: { node: LauncherWidgetNode }) {
  if (node.kind === "text") return <span className="ot-launcher-widget-text">{node.text}</span>;
  return (
    <div className="ot-launcher-widget-row">
      {node.text ? <span>{node.text}</span> : null}
      {node.children?.map((child, index) => (
        <WidgetNode key={`${child.kind}-${index}`} node={child} />
      ))}
    </div>
  );
}

export function LauncherView({
  presentation,
  onActivate,
  onRelaunch,
  onShowPanel,
  onMutate,
  badges,
  widgetState,
  widgetActions,
  onSelectWidget,
  t = (text) => text,
  language = "en",
  interactionTransport,
}: {
  presentation: LauncherPresentation;
  onActivate: (
    epoch: number,
    displayUUID: string,
    session: number,
    revision: number,
    itemID: string,
  ) => void;
  onRelaunch?: LauncherItemCommand;
  onShowPanel?: LauncherItemCommand;
  onMutate?: LauncherMutateCommand;
  badges?: ReadonlyMap<string, LauncherBadgeEntry>;
  widgetState?: LauncherWidgetState | null;
  widgetActions?: WidgetActions;
  onSelectWidget?: (stackID: string, instanceID: string) => void;
  t?: (text: string) => string;
  language?: string;
  interactionTransport?: LauncherInteractionTransport;
}) {
  const interaction = useLauncherInteractions(presentation, interactionTransport);
  const modifiedInput = useRef(false);
  const composingInput = useRef(false);
  const policy = interaction.state?.configured;
  const canUseKeyboard =
    !!policy?.enabled && !!policy.letterNavigation && !!interaction.state?.letterInputAvailable;
  if (!presentation?.visible) return null;
  return (
    <main
      className={`ot-launcher-shell edge-${presentation.edge} theme-${presentation.appearance.theme} material-${presentation.appearance.material}`}
      style={
        {
          "--ot-launcher-icon": `${presentation.iconPx}px`,
          "--ot-launcher-tint": presentation.appearance.tint,
          "--ot-launcher-opacity": presentation.appearance.opacity,
          "--ot-launcher-border-opacity": presentation.appearance.borderOpacity,
          "--ot-launcher-radius": `${presentation.appearance.cornerRadiusPx}px`,
          "--ot-launcher-gap": `${presentation.appearance.itemSpacingPx}px`,
        } as React.CSSProperties
      }
      aria-label="Option Tab launcher"
    >
      <LauncherItemStrip
        key={`${presentation.epoch}:${presentation.session}:${presentation.displayUUID}`}
        presentation={presentation}
        onActivate={onActivate}
        onRelaunch={onRelaunch}
        onShowPanel={onShowPanel}
        onMutate={onMutate}
        selectedItemID={interaction.state?.selectedItemID}
        onReorderTarget={interaction.setReorderTarget}
        badges={badges}
        t={t}
      />
      {canUseKeyboard ? (
        interaction.state?.keyboardMode ? (
          <input
            autoFocus
            className="ot-launcher-letter-input"
            aria-label={t("Type a letter")}
            placeholder={t("Type a letter")}
            onInput={(event) => {
              const native = event.nativeEvent as InputEvent;
              if (native.isComposing) return;
              const text = event.currentTarget.value;
              if (native.inputType === "insertText" && !modifiedInput.current && text)
                interaction.commitLetter(text);
              event.currentTarget.value = "";
            }}
            onCompositionEnd={(event) => {
              const text = event.data;
              composingInput.current = false;
              if (!modifiedInput.current && text) interaction.commitLetter(text);
              event.currentTarget.value = "";
            }}
            onCompositionStart={() => {
              composingInput.current = true;
            }}
            onKeyDown={(event) => {
              modifiedInput.current =
                event.metaKey || event.ctrlKey || event.altKey || event.shiftKey;
              if (event.key === "Escape") {
                event.preventDefault();
                event.currentTarget.blur();
              } else if (
                event.key === "Enter" &&
                policy.enterActivates &&
                !modifiedInput.current &&
                !composingInput.current &&
                !event.nativeEvent.isComposing &&
                event.keyCode !== 229
              ) {
                event.preventDefault();
                interaction.activateSelection();
              }
            }}
            onKeyUp={() => {
              modifiedInput.current = false;
            }}
            onBlur={() => {
              modifiedInput.current = false;
              interaction.setKeyboardMode(false);
            }}
          />
        ) : (
          <button
            type="button"
            className="ot-launcher-keyboard-toggle"
            aria-label={t("Keyboard navigation")}
            title={t("Keyboard navigation")}
            onClick={() => interaction.setKeyboardMode(true)}
          >
            <span aria-hidden="true">⌨</span>
          </button>
        )
      ) : null}
      {interaction.error ? (
        <p className="ot-launcher-interaction-error" role="alert">
          {t(interactionReason(interaction.error))}
        </p>
      ) : null}
      {widgetState === undefined
        ? presentation.widgets.map((widget) => (
            <aside className="ot-launcher-widget" key={widget.id} aria-label={widget.packageID}>
              {widget.status === "ready" ? <WidgetNode node={widget.root} /> : null}
            </aside>
          ))
        : null}
      {widgetState?.visible && widgetActions ? (
        <div className="ot-launcher-widgets" aria-label={t("Launcher widgets")}>
          {widgetState.slots.map((slot) => (
            <aside
              className="ot-launcher-widget"
              key={slot.id}
              aria-label={localName(slot.name, language)}
            >
              {slot.members.length > 1 ? (
                <div className="ot-launcher-widget-choices" aria-label={t("Widget stack")}>
                  {slot.members.map((member) => (
                    <button
                      type="button"
                      className={member.id === slot.selectedID ? "is-selected" : ""}
                      key={member.id}
                      onClick={() => onSelectWidget?.(slot.stackID ?? "", member.id)}
                    >
                      {localName(member.name, language)}
                    </button>
                  ))}
                </div>
              ) : null}
              {slot.state ? (
                <WidgetView state={slot.state} actions={widgetActions} t={t} />
              ) : (
                <span className="ot-widget-status">{t(statusLabel(slot.status))}</span>
              )}
            </aside>
          ))}
        </div>
      ) : null}
    </main>
  );
}

function interactionReason(reason: string): string {
  if (/deliveryUnverified/.test(reason)) return "Interaction delivery is not verified";
  if (/stale/.test(reason)) return "The launcher changed. Try again.";
  if (/busy/.test(reason)) return "The launcher is busy. Try again.";
  return "Launcher interaction unavailable";
}

function localName(name: WidgetLocalized, language: string): string {
  return name[language] || name.en || Object.values(name)[0] || "Widget";
}

function statusLabel(status: string): string {
  if (status === "preparing") return "Loading widget…";
  if (status === "grantRequired") return "Permission required";
  return "Widget unavailable";
}

import { useRef } from "react";
