# Native switcher materials checkpoint

B10 implementation now connects the existing appearance controls to native materials for the window switcher, app switcher and native-Dock window previews. The feature remains unreleased, and B10 stays unchecked until real Wails compositing and visual acceptance are established.

## Behavior

The existing Blur preference requests a native HUD material. Turning it off uses a solid background. The renderer also stays solid when material creation fails, the host is unavailable or the switcher has not reported a valid interior rectangle. Theme, tint, opacity, borders and radius keep their existing independent per-mode settings.

The switcher window still spans its screen. Only the actual switcher panel receives the material; the frontend reports its rectangle in logical points and the native adapter validates containment again. Resize, appearance and presentation changes retire prior geometry. Selection and capture updates retain the same admitted geometry. Reports carry exact presentation/state identity and ordered sequence numbers; material status has its own increasing revision.

Dock previews use backend-owned bounds. Bounds and material are submitted atomically, and native Show establishes content size before material application. A failed material leaves the preview usable. Folder, media, automation and replacement-Dock host policies retain their separate behavior.

Native surfaces bind one host incarnation. Cancellation, retirement, closure and late fade completions cannot mutate a successor. Material views do not handle input, reparent WebKit content or change activation/Space policy. Dock completion delivery is coalesced off the native UI queue so it cannot wait there for the App lock. Cleanup removes the owned effect and restores only properties still belonging to that host/content pair.

## Verification

- All 24 Go packages passed with race detection and coverage; global Go lint reported zero issues.
- JavaScript unit tests passed: 336 desktop, seven shared and four site tests. Workspace lint and production builds passed.
- All 77 desktop Chromium scenarios passed. The new browser scenario checks interior geometry, state-revision reporting, native/solid status and stale-session rejection. An entrance-transform regression led to final rectangle reporting after transitions and animations.
- Injected native tests cover rectangle validation/conversion, role exclusion, scope ordering, cancellation, stale fade completion and content/appearance restoration. Independent native review found no confirmed blocker. Linux/amd64 platform test cross-compilation passed without CGO.
- App regressions cover a blocked native Apply, hide/reopen, missing reporter, native failures, host recreation, resize, and blur changes. Review found and fixed a native-queue/App-lock deadlock path and an old-bounds/new-style race; both have failing-before/passing-after barrier tests.
- Pinned Wails generation succeeded with 99 methods and 58 models. Current arm64 and x86_64 CGO builds passed; both binaries report macOS 14.0 minimum. They were not launched, installed or distributed.

## Remaining acceptance

The native fixture uses inert hosts and views. It establishes native ownership and geometry behavior, not actual desktop blur through WKWebView. Real light/dark rendering, contrast, clipping at mixed scale, visible fade, foreground/Space behavior and physical host destruction/recreation remain pending. No native GUI, input, device or Dock action was performed for this checkpoint.

The remaining replacement-Dock item, animation, gesture, badge and profile-backup work continues separately. This checkpoint is not completion of the full roadmap or a release.
