# Desktop shadcn migration

The user wants the complete Settings and Alt-Tab presentation built from genuine shadcn/ui components, with the existing appearance and behavior preserved. The v0.6.0 UI is the visual contract: no changes to layout, spacing, typography, copy, colors, materials, or window-switching behavior. This is implementation work authorized by the user, not a new visual-design proposal.

## Architecture

Use the official shadcn New York / Radix registry, React 19, and the existing Tailwind 4/Vite configuration. Components remain owned source under `components/ui`, with documented brand styling and narrowly scoped extensions. Complete shadcn semantic color/radius tokens and dark variants; preserve the independent Settings System/Light/Dark preference and existing switcher appearance settings. Use the official Native Select to retain OS picker behavior. Native input types such as file/color remain supported by shadcn Input.

Settings: Button, Input, Native Select, Switch, Checkbox, Radio Group, Slider, Toggle Group, Tabs, Label, Textarea, Collapsible, Card, Badge, Separator, and alerts as appropriate. Custom section layouts, diagrams, previews, and controlled settings persistence remain application code. No claim that every HTML element must become a library component: text, layout, images, and specialized window rendering compose the shared primitives.

Switcher: reuse the same presentation primitives with the existing material/style classes. Do not introduce modal focus traps, Command filtering, duplicate keyboard handlers, or replacement native-window ownership. Preserve bulk/individual actions, app grouping, preview geometry, all window/session identity checks, and existing gesture hit regions.

## Invariants

- All Settings panels remain mounted and hidden when inactive; drafts survive navigation.
- Canonical loading/import disables every control, including Radix controls that do not inherit HTML fieldset disabled semantics.
- Menubar deep links, controlled state updates, settings import/export, permissions, translations, and Settings theme persistence keep working.
- Accessible names and label associations remain intact; switches expose switch semantics and radio groups support keyboard selection.
- Themes remain scoped to each surface. Popover/tooltip content, if used, inherits its originating theme and disabled state.
- User-specific native runtime parameters remain authoritative over shadcn defaults.
- No backend/version/release/website redesign in this change. Do not launch a native app without need.

## Acceptance

Existing frontend unit and browser suites pass with interaction updates only where the public primitive semantics intentionally change. Add focused coverage for composition, busy-state disabling, keyboard slider/radio operation, and independent themes. Compare production-browser Settings, child-editor, and switcher screenshots against untouched main; inspect any differences and correct visual regressions. Build the complete frontend, run lint and relevant Go checks, inspect dependency/config output, and obtain an independent final review.
