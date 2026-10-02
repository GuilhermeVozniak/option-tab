# Shared native capture stress

Explicit macOS test using four animated disposable windows, two production
`preview.Manager` peers and the real identity-bound ScreenCaptureKit source.
It never launches Option Tab, activates an app, posts input, changes preferences,
requests consent or acts on user windows. Screen Recording must already be
available; denied access fails the frame assertions without requesting it.

From the repository root:

```sh
python3 apps/desktop/internal/platform/testdata/capture-stress/run.py --run-native
```

The driver builds committed `HEAD` in a temporary shared clone. Uncommitted
product changes are intentionally excluded; the current probe files are copied
into the clone separately. It prints the evidence directory and retains the
binary/source hashes, build logs, JSON measurements and exit status there.
Normal `go test ./...` does not traverse this `testdata` directory.

Three warmup cycles precede three batches of 30 cycles. Every cycle requires
at least two frames from each exact owned window, no more than four simultaneous
source calls, one peer draining while the other remains active, both peers
draining completely, empty hidden caches and no subsequent delivered frames.
The probe samples its own current Mach RSS, high-water RSS, CPU time, Go heap
and goroutines. Five-second idle samples follow each batch and final closure.
The driver also records external `ps` RSS samples; startup/exit samples can be
zero and must not replace the probe's self-measured settled RSS.

The `passed` result applies to stream/frame/cache/drain assertions. CPU and RSS
are measurements, with no automatic performance threshold. The 180-second
probe deadline bounds the run; it does not silently skip unfinished cycles.
The driver terminates only its owned probe and fixture processes on failure.

Source return follows native `finish` removing its session, `goStreamDone`, and
the Go stream registry's deferred deletion. This supports registry cleanup by
code-backed inference; it is not direct enumeration of OS capture objects.
This test covers the shared capture engine, not packaged App/Wails surface
arbitration, a real preference toggle, permissions being revoked/restored,
window destruction, physical input or Space/display transitions. A short
memory plateau is not proof of long-term leak freedom.
