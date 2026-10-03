# D13 real AX position fixture

Run from any directory on macOS with Accessibility access already granted to the
invoking environment:

```sh
apps/desktop/internal/platform/testdata/preview-drag/run.sh
```

The runner compiles a private temporary bundled NSApplication fixture and a
standalone driver. It launches only that fixture, verifies its announced PID
against the child PID, and passes its two explicit window IDs. The driver checks
the exact fixture bundle and both CG owners before touching either target. The
runner traps exit, terminates/reaps its child, and removes its private build
folder. No generated binary/app/state is stored in the repository.

The driver includes the production `darwin_preview_drag.m`. Its translation unit
alone substitutes the left-button query and makes tap creation, event posting
and pointer warping abort. It does not call native Start. Production code and
synthetic admission policy are unchanged. This proves actual remote AX
Prepare/AXPosition mutation/cancellation/resource cleanup, **not** DragPreview's
capture/handoff, off-panel physical continuation, or hardware drag feel.

Assertions: exact +80/+60 global-point displacement, unchanged size, unchanged
second fixture window, explicit cancel and observed button-up refusing later
writes, and foreground unchanged through fixture launch/movement/cleanup.
Accessibility denial fails explicitly without requesting permissions.
