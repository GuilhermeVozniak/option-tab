# Dock widgets

Widgets belong to a replacement-Dock profile. In **Settings → Dock**, select a profile, add a widget, enable it, and choose which requested data or controls to allow. Adding or installing a widget grants nothing.

The built-in widgets are Clock, Battery, Network and Audio. Network traffic sampling and audio-output selection require separate optional grants. A desktop without a battery reports unavailable battery data. Network status describes the local connection; it does not make Internet requests. Music and Spotify bindings use the existing shared media providers and also require their app-level enablement and consent.

Combine two to four instances into a stack and select its default member in settings. The Dock's stack selector changes the active member for that display session. Hidden members release their provider subscriptions; another visible widget or media panel can keep its shared provider running. A profile supports up to four visible slots, including stacks, and sixteen total instances.

To install a community widget, choose a local `.zip` or `.otwidget` archive and review its name, version, digest and requested capabilities. **Install** saves that exact reviewed content. Enabling an instance and granting its capabilities are separate actions. Package IDs and author-provided names do not verify a publisher. Replacing a package with a different digest does not transfer grants automatically.

Removing an installed package first disables its exact-digest instances and clears their grants. Built-ins cannot be removed from the package catalog. Turning off the replacement Dock, pausing the app or returning to the native Dock retires widget controls and releases their subscriptions.

## Authoring a package

An archive contains `widget.json` at its root and any PNG assets declared in that manifest. The following original example displays battery charge:

```json
{
  "schemaVersion": 1,
  "id": "com.example.battery",
  "version": "1.0.0",
  "minimumAppVersion": "0.4.8",
  "name": { "en": "Battery charge" },
  "description": { "en": "Displays the current battery percentage." },
  "requiredCapabilities": ["battery.read"],
  "root": {
    "kind": "text",
    "binding": {
      "provider": "battery",
      "field": "charge",
      "formatter": "percent"
    }
  }
}
```

The trusted layout vocabulary is `row`, `column`, `text`, `icon`, `progress`, `sparkline` and `button`. Bindings select a fixed typed field and formatter. Commands select a fixed host action; the host supplies the current target and validates the instance, grants and display before dispatch. The manifest cannot name a Go method, provide a native device target, execute scripts, or load remote resources.

Supported capabilities:

| Capability | Access |
| --- | --- |
| `clock.read` | Time with a validated time zone and fixed format |
| `battery.read` | Charge, charging and power source |
| `network.status.read` | Local connection and interface category |
| `network.usage.read` | Bounded aggregate upload/download rates |
| `audio.status.read` | Current output name and supported volume/mute data |
| `audio.output.select` | Explicit selection from current output choices |
| `media.music.read`, `media.spotify.read` | Metadata from the corresponding enabled provider |
| `media.music.control`, `media.spotify.control` | Supported explicit playback and seek controls |

Manifests may declare required and optional capabilities, localized metadata (`en`, `pt-BR`, `es`), and bounded number, boolean, choice or time-zone settings. Free-form text settings are not supported. Clock format choices must resolve to the fixed `shortTime`, `longTime` or `date` formatter. Unsupported or denied fields stay unavailable; they do not become fabricated zero values.

Limits include a 4 MiB archive, 8 MiB expanded content, 64 entries, a 256 KiB manifest, 512 KiB per PNG and 4 megapixels per image. Layouts allow depth eight, 128 nodes, sixteen children per node and 120 history samples. Executable metadata, links, traversal, duplicate or ambiguous paths, undeclared assets and unknown executable fields are rejected.

See the [package/runtime reference](../apps/desktop/internal/widgets/README.md) for the exact catalog, settings and action semantics. The feature branch's [widget integration report](superpowers/reports/2026-09-07-widget-runtime.md) records automated evidence and remaining native acceptance checks. These additions are not included in the published v0.4.8 release.
