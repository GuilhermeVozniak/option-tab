# Windows / Media same-hover selector proposal

## Current gap

An enabled Music or Spotify Dock item is unconditionally composed as `ContentKind == "media"` in `internal/dock/controller.go`; the window source is deliberately skipped. This preserves media safety but removes the existing window-preview path when `Dock.Enabled` is also true. The frontend has a binary content branch and no content selector.

The smallest safe addition is a backend-owned selector which creates a fresh outer Dock presentation session for every accepted content change. A new session is preferable to changing only the current revision: existing `dock:frames` packets carry a session but no revision, capture/input/drag policies are session-scoped, and Wails event dispatch is unordered. Reusing a session would allow a delayed window frame or input policy from the previous content mode to enter the replacement mode.

## Contract

Extend `dock.State` and `DockViewState` with copied `ContentOptions []string` containing only supported choices in deterministic order (`windows`, `media`). Keep `ContentKind` as the selected value. For an exact enabled Music/Spotify app:

- Media enabled, Dock previews disabled: options `[media]`, no selector.
- Media enabled, Dock previews enabled: options `[windows, media]`; initial choice remains `media` to preserve current behavior.
- Media disabled: existing windows behavior.
- Folder items remain `folder` only and never receive these options.

Add `Controller.SelectContent(session uint64, kind string) error`. It validates the current shown session, exact current app candidate, allowed option, current settings/admission epoch, and treats the already-selected kind as idempotent. A real change advances the outer Dock session and immediately publishes the replacement, keeping the selector available while Windows loads. The existing window source has no cancellation port: its one serialized query finishes, its retired-session result is discarded, and only then may the selected content query run. Returning to Media remains immediately possible during that wait. Selecting media performs no window enumeration. Selecting windows uses the current window source/filter pipeline even for a media-provider app; an inventory error remains an error state rather than an empty success.

Expose `SelectDockContent(session, revision uint64, kind string) error` on App. Under `viewMu`, validate the visible outer session/revision and option, then release the lock before invoking the controller. The controller's own session/admission check is the final guard. Do not optimistically mutate `dockState`; only the controller callback publishes the fresh-session state. The new session retires the old media hover owner, capture stream, input target, wheel policy and drag owner using existing `showDock`/`dismissDockLocked` paths.

Frontend `DockPanelHandlers` gains optional `onSelectContent(session, revision, kind)`. `DockPanelView` renders an accessible two-button segmented selector only when both options are present, before the content body. It sends the outer Dock session/revision. It does not manufacture window entries or media state. While awaiting the backend replacement it may disable the selected/pending button, but it must not switch locally. A rejection is shown only if the same outer session/revision remains current.

## Required event fixes

`DockRoute` must keep the outer Dock event stream authoritative:

1. A `media:update` may update media only when the rendered outer state is currently `contentKind === "media"` and contains that exact media session/revision. It must not assign `contentKind: "media"`, because a delayed media update must never switch a Windows presentation back.
2. A `media:hide` must retire only the embedded media owner. It must not set the entire Dock route state to `null`; `dock:hide` is the sole terminal event for the outer presentation. With a content switch, the fresh outer session makes late old-media events fail admission.
3. New-session admission resets frames, native pointer sequence, gesture/region ownership and embedded-media admission before rendering. Existing `DockRoute.acceptShow` already performs most of this; the regression should establish the complete ordering.
4. The backend must emit old-session hide and new-session show/update without relying on delivery order. Frontend tombstones must reject a delayed old show/update/hide/frame after the new session is admitted.

## Files

Backend:

- `apps/desktop/internal/dock/types.go`
- `apps/desktop/internal/dock/controller.go`
- `apps/desktop/internal/dock/controller_test.go`
- `apps/desktop/internal/dock/media_hover.go` and tests if option derivation is kept pure there
- `apps/desktop/app_dock.go` and focused App tests
- generated bindings only after RPC freeze, owned by coordinator

Frontend:

- `apps/desktop/frontend/src/lib/types.ts`
- `apps/desktop/frontend/src/lib/dock-bridge.ts`
- `apps/desktop/frontend/src/dock/DockPanelView.tsx`, `dock.css`, component tests
- `apps/desktop/frontend/src/App.tsx`, App tests
- `apps/desktop/frontend/src/lib/i18n.ts` for Windows/Media selector labels in EN/PT-BR/ES
- `apps/desktop/frontend/e2e/support/fakeWails.ts` and a focused browser spec

## Meaningful tests

Controller/App:

- Both features enabled on exact Music/Spotify item initially publishes media with `[windows,media]`; media-only never calls `Windows()`.
- Select Windows invokes one bounded window query and publishes a new session with exact filtered windows; select Media from it publishes another new session and does not query windows.
- Same selection is idempotent and preserves session/selection. Wrong session, unsupported kind, provider disable, Dock disable for Windows, pause, inactivity, candidate replacement and blocked-query retirement refuse without a late publication.
- Block Windows inventory, switch/retire/reconfigure, then release: the result cannot replace the current mode. Inventory failure is visible and does not become zero windows.
- Every transition retires capture/input/drag/wheel ownership for the old session; media subscription retires on Windows and is created once on Media. Folder paths remain unchanged.

React/browser:

- Selector appears only with two options, uses localized accessible labels, and sends exact outer session/revision/kind without changing content before the backend event.
- Media -> Windows fresh-session event renders real window cards and starts size/region behavior; Windows -> Media removes window controls/regions and renders the exact media owner.
- Reordered old media update/hide and old Dock frames after switching cannot blank or resurrect content. A media hide alone never closes the outer Dock panel.
- Rejected selection is visible only on the initiating session/revision; a late rejection cannot overwrite a newer backend error or replacement state.
- Chromium verifies real segmented-control clicks, no media command or window action caused by selecting a tab, exact RPC arguments, window thumbnail delivery only in Windows mode, and whole-panel resize after each content change.

## Acceptance boundary

These tests prove state and transport behavior. Native acceptance should hover a disposable provider fixture with a disposable window, switch both ways without moving the pointer off the Dock corridor, verify polling stops in Windows mode and restarts once in Media mode, and confirm no focus/action is dispatched by the selector itself. No real Music/Spotify command or permission prompt is required for this selector acceptance.
