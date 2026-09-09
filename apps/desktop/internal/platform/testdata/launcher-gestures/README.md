# Launcher gesture extraction fixture

`TestLauncherGestureNativeExtraction` compiles this Objective-C translation unit with ARC, Cocoa and macOS14 deployment target. It includes production gesture extraction/mailbox code and replaces only the host eligibility lookup with an exact token/display fixture.

The test uses an unattached NSView and NSEvent property subclass. It creates no NSApplication/NSWindow, installs no tap, posts no input and moves no pointer or Dock. Event-type accessors assert they are used only for the corresponding event type.

Checks include copied logical signs/coordinates, precise/coarse admission, magnify/swipe extraction, token acknowledgement, finger-end to momentum-tail ownership, terminal replay, hide/close, policy admission rollback, nonfinite data, and128 regular records plus one reserved cancellation terminal.

This is extraction/state-machine evidence only. It does not establish delivery of physical gestures to an inactive native launcher host, system gesture arbitration, hardware cadence or keyboard focus.
