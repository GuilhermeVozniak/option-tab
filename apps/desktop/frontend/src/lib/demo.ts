// Demo state for visual verification of the overlay (route #demo). Renders the
// switcher with synthetic windows, icons and thumbnails so the UI can be
// screenshotted/compared against AltTab without the native backend. Not used in
// production paths.
import { emptyState, type SwitcherState, type VisualStyle } from "./types";

function svgURL(svg: string): string {
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

function icon(letter: string, c1: string, c2: string): string {
  return svgURL(
    `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${c1}"/><stop offset="1" stop-color="${c2}"/></linearGradient></defs><rect width="64" height="64" rx="14" fill="url(#g)"/><text x="32" y="44" font-size="30" font-family="Helvetica" font-weight="700" fill="white" text-anchor="middle">${letter}</text></svg>`,
  );
}

// preview mirrors shot() at the preview capture's resolution (the real one is
// a 1024px window snapshot), so the #demo route exercises the same
// selected-window preview path the native backend streams.
function preview(label: string, c1: string, c2: string): string {
  return svgURL(
    `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="640"><defs><linearGradient id="b" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${c1}"/><stop offset="1" stop-color="${c2}"/></linearGradient></defs><rect width="1024" height="640" fill="url(#b)"/><rect width="1024" height="64" fill="rgba(0,0,0,0.28)"/><circle cx="40" cy="32" r="12" fill="#ff5f57"/><circle cx="80" cy="32" r="12" fill="#febc2e"/><circle cx="120" cy="32" r="12" fill="#28c840"/><text x="512" y="360" font-size="44" font-family="Helvetica" fill="rgba(255,255,255,0.9)" text-anchor="middle">${label}</text></svg>`,
  );
}

function shot(label: string, c1: string, c2: string): string {
  return svgURL(
    `<svg xmlns="http://www.w3.org/2000/svg" width="320" height="200"><defs><linearGradient id="b" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${c1}"/><stop offset="1" stop-color="${c2}"/></linearGradient></defs><rect width="320" height="200" fill="url(#b)"/><rect width="320" height="26" fill="rgba(0,0,0,0.28)"/><circle cx="16" cy="13" r="5" fill="#ff5f57"/><circle cx="34" cy="13" r="5" fill="#febc2e"/><circle cx="52" cy="13" r="5" fill="#28c840"/><text x="160" y="120" font-size="17" font-family="Helvetica" fill="rgba(255,255,255,0.9)" text-anchor="middle">${label}</text></svg>`,
  );
}

export const demoState: SwitcherState = {
  ...emptyState,
  open: true,
  style: "thumbnails",
  selected: 1,
  activeSpaceId: 1,
  entries: [
    {
      windowId: 1,
      appId: 1,
      appName: "Chess",
      bundleId: "com.apple.Chess",
      title: "Game 2 | Louis - Computer (White to Move)",
      minimized: false,
      hidden: false,
      fullscreen: false,
      icon: icon("C", "#a8742f", "#5a3a18"),
      thumbnail: shot("Chess", "#6b4a2a", "#3a2814"),
    },
    {
      windowId: 2,
      appId: 2,
      appName: "Safari",
      bundleId: "com.apple.Safari",
      title: "Best Time to Visit Japan: Seasonal Trip Guide",
      minimized: false,
      hidden: false,
      fullscreen: false,
      icon: icon("S", "#2aa7ff", "#0a6cff"),
      thumbnail: shot("triptojapan.com", "#bfe0ff", "#7fb8e6"),
      preview: preview("triptojapan.com", "#bfe0ff", "#7fb8e6"),
    },
    {
      windowId: 3,
      appId: 3,
      appName: "Code",
      bundleId: "com.microsoft.VSCode",
      title: "alt-tab-macos - TransitionScheduler.swift",
      minimized: false,
      hidden: false,
      fullscreen: false,
      icon: icon("V", "#3aa0ff", "#1e6fd0"),
      thumbnail: shot("TransitionScheduler.swift", "#262b3a", "#0e1018"),
    },
    {
      windowId: 4,
      appId: 4,
      appName: "Terminal",
      bundleId: "com.apple.Terminal",
      title: "guilherme - zsh - 120x40",
      minimized: true,
      hidden: false,
      fullscreen: false,
      icon: icon("T", "#444", "#111"),
      thumbnail: shot("zsh", "#1a1a1a", "#000000"),
    },
    {
      windowId: 5,
      appId: 5,
      appName: "Notes",
      bundleId: "com.apple.Notes",
      title: "Untitled - Parity checklist",
      spaceId: 2,
      minimized: false,
      hidden: false,
      fullscreen: false,
      icon: icon("N", "#ffd54a", "#f3b300"),
      thumbnail: shot("Notes", "#fff7d6", "#ffe9a8"),
    },
  ],
};

// demoStateFor renders the demo snapshot in a given visual style, used by the
// #demo route to visually verify parity across thumbnails/appIcons/titles.
export function demoStateFor(style: VisualStyle): SwitcherState {
  return {
    ...demoState,
    style,
    // previewSelected on: the demo route is where the selected-window preview
    // is verified visually (and in e2e); the native app streams the capture.
    appearance: { ...demoState.appearance, style, previewSelected: true },
  };
}
