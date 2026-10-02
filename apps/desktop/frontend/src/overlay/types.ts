import type { SwitcherGestureReporter } from "../lib/useSwitcherGestureRegions";

// OverlayHandlers is the full set of user intents the overlay can emit; the
// shell (App.tsx) wires them to the Go controller through the bridge.
export interface OverlayHandlers {
  onGestureRegions?: SwitcherGestureReporter;
  onAdvance: () => void;
  onReverse: () => void;
  onConfirm: () => void;
  onConfirmWindow: (windowId: number) => void;
  onCancel: () => void;
  onSelect: (index: number) => void;
  onSearchChange: (query: string) => void;
  onClose: (windowId: number) => void;
  onMinimize: (windowId: number) => void;
  onFullscreen: (windowId: number) => void;
  onQuit: (appId: number) => void;
  onHide: (appId: number) => void;
  onAction?: (
    kind: "newWindow" | "forceQuit" | "closeAll" | "minimizeAll",
    windowId: number,
    appId: number,
  ) => void;
  onSelectApp?: (appId: number) => void;
  onSelectAppWindow?: (windowId: number) => void;
  onConfirmApp?: (appId: number) => void;
}
