# Audio and community widget prerequisites

This checkpoint adds the typed audio-output backend and an inert local widget-package store. They are prerequisites for retained H12/H13/H14; no widget runtime, installation UI, grant editor or output-selection UI is claimed complete here.

The audio service owns a cancellable, joined CoreAudio listener lifetime, immutable/coalesced snapshots and one guarded output selection at a time. Selection checks current observation generation and exact live output UID before and after preparation, writes only DefaultOutputDevice and confirms the resulting UID. Unsupported volume/mute stay unavailable. Listener failure retires the source. It does not open streams, request microphone access or select a device from observation.

The widgets package validates a closed declarative manifest and bounded ZIP archives. Fixed layouts, typed fields/formatters and declared command enums confer no executable authority. Installation grants nothing. Private staging and atomic publication save verified content under its digest; reads revalidate it and return independent immutable values. Preview and install use caller-owned readers and cannot forcibly interrupt a reader blocked inside Read. The caller supplies an app-owned directory; this is not a sandbox against arbitrary writers with the same OS identity.

The package README documents the format, limits, capability catalog and APIs, and includes an original clock manifest example. Media declarations are metadata for later integration; this checkpoint does not dispatch player commands.

Focused race tests, platform/widgets lint and Linux platform test compilation passed. The combined Go race/coverage run passed all 23 packages after merging the profiles checkpoint. Independent bounded reviews found no concrete blocking issue in audio lifetime/identity guards or the widget package/store boundary. Native tests replace HAL operations and do not query or switch a real device.

Real HAL listener delivery, driver behavior, device selection, explicit native package selection, instance grants/subscriptions, widget rendering/stacks and the complete retained roadmap remain ongoing work. No release, installed application, native Dock preference, cursor or audio device was changed.
