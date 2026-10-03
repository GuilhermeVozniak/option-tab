# Desktop UI components

This directory contains shadcn/ui source installed with **shadcn 4.21.1**, using the
**New York / Radix** registry. `components.json` configures the Vite alias, Tailwind
4 stylesheet, neutral base color, and Lucide icon library. The locked `shadcn`
dev dependency supplies the shared Tailwind utilities and the maintenance CLI.

The existing Option Tab design is intentional. Registry components are customized
through the same local-source ownership model as shadcn. Settings retains its
neutral System/Light/Dark palette; runtime switchers retain their materials,
accents, and native interaction behavior.

## Components and application boundaries

- NativeSelect/NativeSelectOption retain OS dropdowns, empty/default option values,
  and native change events. They are the official shadcn Native Select component.
- Switch is used for toggle-shaped settings; Checkbox is used for square widget
  checkboxes. Both expose their actual accessible roles and Radix state attributes.
- RadioGroup, Slider, Tabs, ToggleGroup, Collapsible, Label, and Separator use Radix
  primitives. Slider labels/descriptions are forwarded to the focusable thumbs.
- Button and Badge support Slot `asChild`. Card also supports `asChild` so native
  switcher `li`, `article`, and `section` elements keep their existing hierarchy.
- Button/Badge `variant="unstyled"` and Card/Tabs/ToggleGroup/RadioGroup/Label/Alert
  `appearance="unstyled"` preserve custom presentation while retaining composition
  and interaction. Existing glass/status variants and compact default dimensions
  are deliberate brand extensions, not placeholder approximations of shadcn.
- FormDisabledProvider carries canonical loading/import state into controls that
  cannot inherit HTML fieldset disabling. Nested providers cannot re-enable an
  ancestor's disabled form. Keep the native fieldset as well.
- Application-specific layout, window images, native gesture hit regions, and
  non-key-window keyboard handling stay in the app. Do not introduce modal Dialog
  or Command key ownership into the native Alt-Tab panel.

## Themes

`src/styles.css` imports Tailwind, `tw-animate-css`, and `shadcn/tailwind.css`, defines
semantic color/radius utilities, and supplies runtime surface tokens. Settings
maps those semantic tokens to its existing palette in `settings/settings.css`.
The Settings root resolves `.dark` independently and preserves
`option-tab.settings-theme`; it never changes the document theme or exported
switcher preferences. New portaled controls must inherit their originating
surface's theme rather than relying on document defaults.

The application utility `lib/utils.ts` re-exports shadcn's `cn` merger so existing
imports and registry components use one class-merging implementation.

## Adding or updating components

From `apps/desktop/frontend`:

```sh
bunx shadcn info
bunx shadcn add <component> --dry-run
bunx shadcn add <component> --diff
```

Review generated changes before overwriting customized source. Preserve brand
variants, disabled-context propagation, scoped themes, and accessible names.
Use existing browser and unit suites plus before/after screenshots for any DOM or
style change. Native Select inputs use ordinary change events; Radix switches and
radio groups use checked/value callbacks, and sliders use number-array values.
Do not restore the removed bespoke `select`, `radio`, or `segmented` wrappers.

Official references:
- https://ui.shadcn.com/docs/components
- https://ui.shadcn.com/docs/components/radix/native-select
- https://ui.shadcn.com/docs/theming
- https://ui.shadcn.com/docs/cli
