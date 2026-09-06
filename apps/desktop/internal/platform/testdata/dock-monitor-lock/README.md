# Dock monitor lock acceptance

Run from apps/desktop:

```sh
go test -race ./internal/platform -run 'Test(DockLock|NativeDockLockCallback|NativeDockLockPlacement|NativeDockLockJoined|NativeDockLockQueued)' -count=1
OPTION_TAB_DOCK_LOCK_READONLY=1 go test ./internal/platform -run '^TestNativeDockLockReadOnlySnapshot$' -count=1 -v
```

The first command uses isolated native handles and no real tap or movement. The second only reads Dock AX/display inventory.

Separately authorized inert transport diagnostic (installs an active tap briefly, posts only an inert tagged null event, drops it without conversion, checks cursor/foreground unchanged, joins cleanup):

```sh
OPTION_TAB_DOCK_LOCK_NULL_TRANSPORT=1 go test ./internal/platform -run '^TestNativeDockLockInertTransport$' -count=1 -v
```

Do not run this alongside another agent's desktop acceptance. Public automatic placement is unavailable. None of these commands establishes actual physical edge protection or Dock relocation acceptance.
