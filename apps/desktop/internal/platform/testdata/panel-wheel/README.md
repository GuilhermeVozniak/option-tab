# Isolated native Dock panel wheel seam

From the repository root:

```sh
go test -race ./apps/desktop/internal/platform -run 'TestDarwinPanelWheel|TestDockPanel' -count=1 -v
```

`TestDarwinPanelWheelNativeSeam` compiles `main.m` using the installed Xcode clang into Go's temporary test directory and runs it with a 20-second timeout. No fixtures/binaries remain in the repository.

The native executable includes the production panel implementation and tests its exact classifier, fixed-capacity queue, policy, and validator. An NSEvent subclass supplies copied event properties to the production extraction path, with an unattached NSView for coordinate conversion. It does not create/show NSWindows, register a CGEvent tap, send OS input, activate applications, or perform window actions.

Coverage: precise phase admission; coarse, region misses and disabled pass decisions; immutable window/PID; inverted delta normalization; exact session/revision validation; cancellation on hide/policy change; pending-ack backpressure; acknowledgment during a stream; owned momentum after finger end; 250ms tail expiry; bounded queue overflow preserving all 128 admitted packets and a reserved cancellation packet.

This is native classification/transport evidence. It does not prove physical trackpad finger count, end-to-end WK scrolling on the pass-through branch, or integration actions. Production `sendEvent:` calls `[super sendEvent:event]` with the original event only when the classifier declines ownership, and suppresses owned events without direct WK responder forwarding.
