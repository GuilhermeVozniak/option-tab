# Distribution Contract Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this bounded plan inline. Coordinator has already authorized implementation; no additional execution-choice pause or commits.

**Goal:** Make future macOS releases universal, explicitly macOS 14+, and consistent across packaging, updater and website without changing existing published download links.

**Architecture:** Exact release filenames are the contract. Future macOS assets use `option-tab_VERSION_darwin_universal.dmg`; the updater prefers a unique exact architecture asset and otherwise a unique exact universal asset. Website publication metadata stays independently pinned to v0.4.8 ARM64 until a real universal artifact exists. Release packaging fails closed without signing/notarization prerequisites; unsigned development packaging is an explicit separate mode.

**Tech Stack:** Go updater, TypeScript shared/site helpers, Bash packaging, Taskfile, GitHub Actions, Xcode build/inspection tools.

**Spec:** `docs/superpowers/specs/2026-09-06-dockdoor-parity-design.md` milestone 7; retained G01/G03. Coordinator decisions: macOS14 explicit floor; universal future releases; preserve v0.4.8 ARM64 downloads.

## Global constraints

- Work only in `.worktrees/parity-followups`; no release, version bump, signing credentials, installation, git staging or commits.
- Do not generate bindings or edit desktop frontend here. Build commands must generate through the repo-pinned Wails CLI before frontend compilation, without stale-binding fallback; coordinator executes actual generation/build later.
- G02/G04–G06 and replacement Dock are outside this slice.
- Compilation is not Intel/native compatibility acceptance. Publication still requires signed/notarized package and supported-hardware evidence.

### Task 1: Freeze exact asset selection and published download metadata

Files: `packages/shared/src/index.ts`, its tests; `apps/desktop/internal/update/update.go`, its tests; `apps/web/lib/download.ts`, its tests; both download components and relevant site assertions.

- [x] Add failing tests for universal filename, unique architecture preference, universal fallback on ARM64/AMD64, duplicate refusal, wrong version/extension rejection, and unchanged v0.4.8 ARM64 site URL.
- [x] Run focused Vitest and updater Go tests; record RED.
- [x] Extend `Arch` with universal (Darwin only); keep `Release.AssetFor(platformArch string) string` and return empty for missing/ambiguous candidates. Match full filename including current release version and format, never substring.
- [x] Add explicit published macOS architecture metadata to site download helpers and consume it from both components. Keep APP_VERSION=0.4.8 and published architecture=arm64. Future metadata can select universal only after upload verification.
- [x] Run the focused tests and TypeScript/Biome checks.

### Task 2: Make packaging order, architectures and support floor truthful

Files: `scripts/bundle.sh`, narrowly factored binding/build helpers if needed, `Taskfile.yml`, active `Info.plist`, packaging contract tests.

- [x] Add controlled fake-tool tests that exercise packaging without generating bindings/building UI/signing: generation occurs before frontend compilation; failed generation aborts; release without credentials aborts; universal slices and resulting architecture inspection are mandatory; unsigned mode is explicit and correctly named.
- [x] Run RED, then implement pinned CLI resolution from go.mod and `go run <CLI>@<resolved-version> generate bindings`. Never accept a globally installed arbitrary CLI or silently use stale files.
- [x] Set MACOSX_DEPLOYMENT_TARGET=14.0 and matching CGO compile/link deployment flags for both slices. Release always builds arm64+amd64, verifies with lipo, and emits darwin_universal.dmg. Local unsigned mode may select a host architecture but must name it accurately.
- [x] Set bundle minimum 14.0. Assert packaged minimum/version/architecture before signing. Preserve dictionary resources.
- [x] Run shell syntax, controlled packaging tests and metadata validation. Do not execute real binding generation, signing or installation.

### Task 3: Gate release workflow and document remaining acceptance

Files: `.github/workflows/release.yml`, release/build documentation, website support-floor copy.

- [x] Require signing/notarization inputs for the macOS release job before expensive work; retain explicit unsigned developer builds outside release publication.
- [x] Make the release job invoke universal release mode and publish only the expected artifact, not stale wildcard DMGs. Keep Windows/Linux demo packaging accurately separated.
- [x] Ensure generation precedes frontend compilation in Taskfile and workflow paths. Failures must stop the pipeline.
- [x] Update supported-version/download/build documentation to 14+, explain existing ARM64 publication vs future universal assets, and document architecture/signature/notarization/update acceptance commands.
- [x] Run focused contract tests and lint. Report exact passing checks and outstanding coordinator builds, Intel/Apple Silicon runtime validation and signing/notarization gates. Do not mark G01/G03 native acceptance complete.

## Verification and acceptance boundary

Implementation and focused contract checks are complete. Packaging assertions cover
actual requested Mach-O architecture/minimum inspection and assembled plist version,
minimum OS, executable and scripting dictionary metadata. The fake-tool harness does
not establish real architecture, signature or notarization acceptance.

The coordinator additionally requested a real clean-checkout generator check. A
separate `git archive HEAD` tree with no frontend/dist successfully ran the pinned
v3.0.0-alpha2.117 CLI: 1 service, 74 methods. Dist remained absent; no bootstrap or
stale fallback was required. Current live bindings were not touched by this worker.

See `.superpowers/sdd/2026-09-07-parity-followups/distribution-report.md` for commands,
red/green evidence and remaining real-build/hardware/signing gates.
