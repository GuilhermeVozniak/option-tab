# Native Dock input receiver smoke

This opt-in fixture creates only a disposable nonactivating panel. It posts one
synthetic down/up into its own receiver, verifies production input tap pass-through,
then restores the saved cursor. It neither invokes app actions nor changes Dock
preferences. Coordinate exclusive desktop input before running.

From `apps/desktop`:

```sh
clang -fobjc-arc -framework Cocoa -framework ApplicationServices internal/platform/testdata/native-dock-input/receiver.m -o /tmp/option-tab-input-receiver
OPTION_TAB_INPUT_RECEIVER=/tmp/option-tab-input-receiver go test ./internal/platform -run TestNativeDockInputReceiverSmoke -count=1 -v
```

The test launches/terminates only that exact child process and stores state under
its temporary test directory. Production filtering intentionally excludes synthetic
events. This proof therefore establishes real tap installation/lifecycle and
synthetic pass-through, **not physical-device qualified gesture recognition**.
The isolated callback tests separately prove suppression/replay decisions.
