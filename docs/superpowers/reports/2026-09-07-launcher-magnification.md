# Launcher magnification

Implementation checkpoint for H06. Each replacement-Dock profile can enable spring magnification and choose scale from 1 to 2 and reach from 0 to 4 neighbors. Existing profiles default to disabled, scale 1.35 and reach 2. Disabling preserves the configured values. Settings copy includes English, Portuguese and Spanish.

The native layout reserves the largest transformed envelope before showing the host, including all configured group members. Resolved logical-point insets account for scale, neighbor displacement, spacing, shell borders and padding. Layout reduces the requested scale when necessary to fit the display, protected rectangles and the existing 32-point native-Dock recovery margin. Disabled or scale-1 profiles retain the prior bounds and reveal band. Child-panel placement consumes the resolved parent bounds.

One animation loop applies a critically damped spring using actual frame elapsed time. Transforms affect only pointer-inert visuals; item buttons retain stable hitboxes. Aggregate expansion is bounded even when pointer movement leaves several springs settling at once. Clock-only content revisions retain animation ownership. Opening groups refreshes the current button set. Scrolled content conservatively suppresses expansion whenever a partly visible icon would acquire additional clipping or displacement; ordinary scroll clipping remains unchanged.

Animation stops at rest, on hidden document, disabled/retired owner or unmount. Reduced-motion preference immediately restores scale 1. Pointer leave settles without activating an item. No hover action, haptic or cursor movement is introduced.

## Verification

- All 25 Go packages pass race/coverage tests; golangci-lint is clean.
- 385 desktop, 7 shared and 4 website unit tests pass. Biome, TypeScript and production builds pass.
- The full desktop Chromium suite passed 86 checks. After the scroll-edge review fix, five focused magnification checks passed, covering all four edges, opened groups, stable hitboxes, reduced motion and a real partial-item scroll position.
- Pinned Wails generation reports 122 methods and 68 models.
- Real CGO arm64 and x86_64 builds pass; `vtool` reports minimum macOS 14.0 for both. Binaries were not launched or distributed.
- Regressions cover actual 60/120-Hz time steps, settling and aggregate limits, lifecycle cancellation, strict optional defaults and copies, version-1 rejection, negative display origins, logical scale invariance, group envelopes, protected rectangles and reduced available geometry.

Review identified and fixed version-1 acceptance of the later magnification field and additional clipping of partially visible scroll-edge items. The version-1 fixture was updated to omit the later field. An older focus-loop test also exposed a scheduling race: it opened the switcher before the focus event reached MRU, suppressing its own input. It now waits for actual MRU publication and passed 100 race runs.

An initial App test build overlapped Vite's replacement of embedded assets; the final Go gate ran against stable frontend assets. One JavaScript test exceeded its timeout under concurrent load, passed immediately in isolation, and passed in the final full suite. No timeout was increased and no hook was bypassed.

## Remaining acceptance

Physical high-refresh smoothness, native nonactivating WebKit pointer delivery, mixed-scale displays and native recovery geometry remain acceptance gates. Browser simulation and pure layout tests do not establish those outcomes. Runtime dragging/grouping, gestures/navigation and supported badge sources remain retained work. No new release is part of this checkpoint.
