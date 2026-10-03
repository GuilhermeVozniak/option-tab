# Isolated automation AX seam

`go test ./internal/platform -run TestAutomationNativeActiveAndFinalGuardSeam`
compiles this translation unit with the production `darwin_active_window.m`.
File-private function pointers replace AX attribute/actions, root/window/process
identity checks, trust and CG descriptions. The executable posts no event and does
not install a tap or show a window. It creates inert AX application references
only to exercise actual CF type/refcount behavior; there are no remote AX reads
or writes in the substituted calls.

The fixture establishes:

- exact root/owner membership, no CG-only retained surface acceptance;
- focused app/window evidence and reread, one retry for focus churn;
- cancellation, process replacement and permission revocation after preparation;
- minimize always sets true, fullscreen uses its explicit boolean;
- native unsupported/rejected actions remain errors;
- independent CF ownership is released at exit.

It does not establish real Accessibility/TCC grants, actual AX focused-window
behavior for a particular app, native focus acceptance, visible postconditions,
off-Space root availability, physical input, or Apple-event transport. Real
mutation acceptance requires coordinator approval and only an announced disposable
fixture PID/window identity. No user apps, cursor or Dock may be controlled here.
