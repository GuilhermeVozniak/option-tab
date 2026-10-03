# Shared capture lifecycle stress

The production shared capture engine completed 93 native start/stop cycles on
macOS 27.0 (26A428), arm64. Product sources were isolated at
`8e9d43700911c5fce1b8c72706c9a5f34a571396`; the probe was added separately.
No Option Tab build was launched, no user window was targeted, and no input,
permission request or preference change was sent.

Four animated disposable windows were captured through two production
`preview.Manager` peers and the real identity-bound ScreenCaptureKit source.
Each cycle required at least two frames from each window, retired one peer
while the other remained active, then retired both. Three warmup cycles were
followed by three batches of 30 cycles.

## Results

- 372 source lifetimes started and returned; peak concurrency was four.
- All 1,773 frames came from the owned fixture windows. Each cycle returned to
  zero active source calls, cleared both export caches and delivered no frame
  during the subsequent idle check. No native stream error was observed.
- Current RSS was 18.55 MiB before capture and 45.38, 46.72 and 46.97 MiB after
  the three batches and their five-second idle periods. It remained 46.97 MiB
  after final closure and another five-second idle period. This is a bounded
  observation of memory settling, not proof of long-term leak freedom.
- The final five-second idle period used 0.000228 seconds of probe CPU time.
  The entire 137.06-second probe used approximately 23.24 CPU seconds, including
  setup, capture, teardown and idle. Fixture and WindowServer CPU are excluded.
- Both owned processes exited; the driver returned zero. An independent
  source/evidence review found no blocking defect for these scoped assertions.

The automated `passed` field covers frame, cache, concurrency and drain
assertions. CPU/RSS are measurements without an automatic pass threshold.
Current RSS is sampled inside the probe with Mach task information; external
`ps` startup/exit samples are not used as settled memory evidence.

Source return follows native `finish` removing its session before
`goStreamDone`, then Go's deferred stream-registry deletion. This supports
registry cleanup by code-backed inference. It does not directly enumerate
operating-system capture objects.

## Reproduction and evidence

The retained [native probe](../../../apps/desktop/internal/platform/testdata/capture-stress/README.md)
requires explicit `--run-native`, uses committed product sources in a temporary
clone and retains logs, source/binary hashes and measurements. Its Go probe and
fixture match the completed run, with formatting-only changes to the Go file.
The retained driver's syntax, repository resolution and explicit-run refusal
were checked separately; the completed native run used the equivalent
development driver.

Local evidence is under
`.superpowers/sdd/capture-stress-2026-10-02/extended/`: `probe.jsonl`,
`run.json`, `verified-metrics.json`, build/driver logs and a 246-file source
hash manifest. Probe executable SHA-256:
`daba023ebb1c5034e7404076395f6937a5687d95508e8c7ab922f17ae52fbce0`.

An earlier September 15 run was interrupted before completion and is not a
pass. The first October 2 run completed 33 cycles, but its external final RSS
sample caught process exit. The extended run added direct current-RSS sampling
and completed all 93 cycles.

## Remaining C06 acceptance

C06 remains unchecked. This exercises the shared managers and native capture
engine, not packaged App/Wails Dock-to-keyboard surface arbitration or a real
live-preview preference toggle. Permission revocation/regrant, window
destruction, inaccessible AX roots, Space/display transitions and the broader
physical UI matrix need their own acceptance evidence.
