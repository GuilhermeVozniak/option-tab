# Launcher profile export/import — H17

Approved retained scope: export/import Dock profiles, items and widget settings. Implement after the H09/H11 panel checkpoint. Coordinator owns Git, generated bindings and shared App wiring.

## Format and behavior

Export the selected profile as a bounded JSON document using the existing Settings JSON download/file-input flow. Import adds a new, unassigned profile; it never replaces the current profile or enables the replacement Dock. The user chooses its display assignment with the existing settings control.

Use envelope `{format:"option-tab.launcher-profile",version:1,profile:...}`. The profile retains layout, appearance, item order/groups/links, folder view, widget package+digest references, typed widget settings and stacks. Bound the entire UTF-8 document to256KiB and reuse strict configuration validation; reject trailing JSON, duplicate/unknown fields, unsupported versions and all invalid profile/item/widget limits.

Transfer contains no security bookmarks, selected paths, native handles, provider samples, custom icon bytes/IDs, grants or enabled widget execution. Export clears every widget's Enabled and Grants; import independently clears them again, regardless of document content. Widget package installation remains the existing explicit flow. Preserve package/digest/settings references and explain that widgets must be enabled again in Settings.

Each app/folder/file item retains its kind/label/order but gets a structural `selection-<digest>` placeholder, never an ID accepted by the private native reference store. On import allocate fresh placeholders and a fresh profile ID so importing the same document twice cannot restore old native authority. Existing explicit Choose/Relink controls repair these selections. Links keep canonical validated http(s) destinations and never open on import. Strip IconID; explain custom icons stay on the source Mac. Do not embed display UUID bindings or focused-app rules; those remain configured locally.

No network access, permission prompt, app/file activation, chooser, package installation or provider subscription during parse/review/import. Import saves through the existing serialized settings writer and preserves unrelated settings. Eight-profile capacity refusal is explicit. A failed save retains the current settings and leaves no private reference/icon artifacts.

## Contracts

Pure config helpers in `internal/config/launcher_transfer.go`:

```go
ExportLauncherProfile(profile LauncherProfile) ([]byte,error)
ParseLauncherProfile(data []byte) (LauncherProfile,error)
```

Both validate and sanitize authority-bearing fields independently; neither performs I/O nor manufactures native reference authority. Use deterministic structural placeholders in exported data; App creates fresh IDs for the imported profile and selections. Stack/member relationships within the new profile remain intact.

App bridge in `app_launcher_profile_transfer.go`:

```go
GetLauncherProfileExport(profileID string) (string,error)
PreviewLauncherProfileImport(document string) (LauncherProfileImportReview,error)
ImportLauncherProfile(document,digest,expectedRevision string) (LauncherProfileImportResult,error)
```

Review: digest of exact bounded bytes, revision of current ReplacementDock settings, safe profile name, item/widget counts and fixed repair notices. It parses without saving. Apply reparses/rechecks exact digest, checks current preferences admission and CAS under saveMu, appends a sanitized profile to the latest full settings, and saves once. Return new ProfileID and canonical SettingsJSON for recovery if the normal reload fails. No renderer-provided paths, native selection IDs or grant tokens become authority.

Export is read-only. Import mutations require the existing preferences/session admission, checked again immediately before the serialized save. Keep native and AppKit work out of this path. Use a context/owner only if existing preferences lifecycle requires it; do not add an installer or background polling owner to a JSON transform.

## Task 1 — portable config boundary

- RED/GREEN export/import round-trip preserves appearance/layout/order/group/stack/widget typed settings while stripping icons/grants/enabled execution and replacing selected references.
- Adversarial tests: a known local32-hex reference in imported data never survives; malicious enabled/granted widgets become inert; malformed URLs, invalid package/digest/settings, duplicate keys, oversized UTF-8, unknown fields, trailing JSON and capacity/profile limits refuse.
- Verify copies do not alias existing config slices/maps. Reuse existing strict decoding and profile validation; keep exports deterministic.

## Task 2 — App review/apply

- RED/GREEN review performs zero save/native/source calls; changed document digest or Dock revision refuses; closed preferences/inactive session refuses; persistence failure preserves prior full settings; unrelated settings are preserved; repeated imports get fresh IDs and unassigned profiles; ninth profile refuses.
- Serialize with saveMu and the existing saveSettingsLocked path. Persist no native selection/icon files. Return a copied canonical result.

## Task 3 — preferences flow

- Add Export profile and Import profile beside existing profile controls. File input checks byte size before reading; Go review precedes rendering untrusted content. Review shows safe profile name, counts and short repair guidance; explicit Import commits the exact reviewed text/digest/revision. Cancel discards the local document.
- Reuse useSettingsModel.mutateSettings to wait for pending saves, block simultaneous full-setting edits/imports and refresh canonical settings after commit. Select the imported profile in the editor without assigning/enabling it. Use returned canonical settings if post-save reload fails.
- EN/PT/ES, accessible controls, scoped stale completions, no raw permission/capability identifiers in product copy. Tests cover cancel, changed profile/editor, pending save, stale preview, error recovery and repaired selections.
- Chromium: download contains sanitized profile; import review/apply yields a new unassigned profile with inert widgets and missing selections. Native WK file-input/download acceptance remains separately recorded, consistent with existing Settings transfer.

## Gate

Focused config/App race tests, frontend tests/build/lint and actual Chromium flow. Root regenerates bindings once signatures settle and runs required repository checks. Record implementation separately from native file-dialog acceptance; no release or merge as part of H17.
