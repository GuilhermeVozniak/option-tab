# Replacement Dock profiles, geometry and appearance

Continue the approved full retained scope from foundation commit `4d0dc9c`; no release or scope reduction. This slice implements profile management and the H02/H07/H08 layout/appearance controls. Focus rules, persistent items, magnification and the other retained H features follow independently. Existing lifecycle, recovery, declarative grants and native action guards remain authoritative.

## Frozen configuration and presentation contract

Replacement settings version becomes 2. Load version 1 only through an explicit validated migration that preserves global enablement, every binding/profile/widget/grant and existing dimensions. Validate legacy values before supplying new defaults; reject future versions and unknown/duplicate fields. Perform the final migrated assignment after top-level JSON decoding so that decoding cannot overwrite the migration. Top-level settings version remains unchanged.

Existing profile fields retain their bounds. `edge` accepts bottom/left/right/top; `layout` accepts floating/fullWidth; new `alignment` accepts start/center/end (default center). Add a value field `appearance`:

```go
type LauncherAppearance struct {
    Theme string           // system|light|dark; default system
    Material string        // solid|system; default solid
    Tint string            // #RRGGBB only; default #172033
    Opacity float64        // finite .35..1; default .76
    BorderOpacity float64  // finite 0..0.5; default .16
    CornerRadiusPx int     // 0..28; default 18
    ItemSpacingPx int      // 2..20; default 6
    ShowLabels bool        // default true
}
```

Profiles/bindings stay bounded at 8. New configuration fields are all active in this slice; no unused magnification/item fields. `launcher.Presentation` gains resolved `Edge`, `Layout` and `Appearance` fields with matching lower-camel JSON names. No new RPC is required for ordinary profile edits; existing SaveSettings validation/persistence remains authoritative.

## Layout and native styling

Generalize the pure layout through along-edge/cross-edge coordinates. Intersect full/usable frames, reserve the native Dock access corridor and protected bounds, then apply profile inset. Floating content length includes bounded item/widget size and configured spacing; fullWidth occupies the safe available span. Align floating layouts start/center/end. Left/right transpose axes; top grows inward below safe/menu areas. Overflow stays inside fixed host bounds. Reveal bands stay on the outward side of the inset launcher, never at the physical screen edge. Unknown or impossible geometry yields.

The native environment reader intersects NSScreen visibleFrame with its public safeAreaInsets before emitting UsableFrame. Do not expose top placement based on an untested assumption that an auto-hidden menu bar removes a notch. Native seam tests cover the conversion; notch hardware acceptance remains recorded separately.

Add a narrow optional platform `LauncherPanelStyler` with `SetLauncherStyle(LauncherPanelStyle) error`; style has Material, Theme and CornerRadiusPx. It targets only replacement panels and rejects retired/non-launcher tokens. The native adapter uses NSVisualEffectView behind Wails content for system material, an explicit appearance for light/dark, and clips to the configured corner radius. Solid removes native material. Restore the original Wails content and its frames on close. Existing preview/media panel behavior is unchanged. App applies the immutable style during creation before first Show, outside owner locks. A requested native style failure is reported through the existing host-failure path.

The trusted renderer maps typed appearance values to CSS; no arbitrary style strings. Native material provides blur; CSS handles tint, opacity, border, spacing and labels. Theme system follows prefers-color-scheme; explicit light/dark has legible text. Vertical layouts scroll on their primary axis. Host dimensions are backend-owned; no DOM resize feedback.

## UI and ownership

Profile selector plus New/Duplicate/Rename/Delete. Stable generated IDs; duplicate names are allowed. Never delete the final profile. Delete-and-reassign updates bindings atomically using an explicitly selected remaining profile. Separate display assignments from profile editing, support removing/re-adding dynamic main or UUID assignments, retain disconnected UUIDs and show runtime names/conflicts. Respect all bounds before emitting edits. Keep recovery visible and clock opt-in per profile. Describe clock permission in product language (“Reads local time while visible”), without capability identifiers in ordinary controls. EN/PT/ES strings and accessible labels.

- Pure worker owns config/replacement.go and tests, the necessary config.go decode hook, launcher layout/reconcile/types and focused tests. No App/frontend/native edits.
- UI worker owns frontend sources/tests/e2e, including hand-written types and fixtures, excluding generated bindings. No Go/native changes.
- Native worker owns native launcher/panel implementation and platform tests plus safe-area observation. Root owns the shared style interface, App/main/host adapter and related App tests.
- Root owns git, generated bindings, integration and combined gates. Work in the existing replacement-dock worktree. No GUI run, cursor/Dock movement, preferences mutation, app activation or output-device change during automated checks.

## Verification

Start with failing behavior tests for v1 migration and malformed/future settings, profile isolation, all four edges/layouts/alignments, negative origins/portrait/scale, protected bounds and overflow. Check profile CRUD/delete reassignment/disconnected bindings and save failure; renderer tests verify resolved appearance and axis behavior. Native seams verify style-only launcher targeting, retired handles, content restoration, safe-area math and no activation. Root validates App style application before Show and exact-host failure cleanup, generates bindings, runs combined gates once stable, and updates the draft PR checkpoint. Physical acceptance remains separate from seam results.
