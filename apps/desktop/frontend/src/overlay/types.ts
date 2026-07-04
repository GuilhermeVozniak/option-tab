// OverlayHandlers is the full set of user intents the overlay can emit; the
// shell (App.tsx) wires them to the Go controller through the bridge.
export interface OverlayHandlers {
  onAdvance: () => void;
  onReverse: () => void;
  onConfirm: () => void;
  onCancel: () => void;
  onSelect: (index: number) => void;
  onSearchChange: (query: string) => void;
  onClose: (windowId: number) => void;
  onMinimize: (windowId: number) => void;
  onFullscreen: (windowId: number) => void;
  onQuit: (appId: number) => void;
  onHide: (appId: number) => void;
}
