# Conditional Music lyrics implementation plan

> **For agentic workers:** Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` task by task; preserve the existing approved media architecture.

**Goal:** Complete E05's provider source by displaying synchronized lyrics when the already enabled, permitted Music application supplies timestamped text.

**Architecture:** An optional provider capability shares the existing media gate and exact track/process scope. The App loads a chosen local association first, then conditionally reads Music into the existing bounded LRC parser and timeline. The panel distinguishes the source without adding a service, polling loop or permission prompt.

**Tech Stack:** Go, Objective-C typed AppleEvents, React/TypeScript; existing test fixtures and Playwright.

**Spec:** [Approved parity design, folder and media panels](../specs/2026-09-06-dockdoor-parity-design.md). This extends the local-file baseline in [the media plan](2026-09-06-dock-media.md).

## Constraints

- Existing media master/provider enable and explicit Automation grant are required. Never launch Music, activate it, request permission implicitly, execute scripts or add a lyrics network service.
- Capture exact PID, process incarnation, generation, track epoch and track ID. Reject stale work before the lyric property read and before publishing.
- Use one total two-second native deadline and a 1 MiB UTF-8 payload cap. Native descriptor allocation precedes that cap; do not claim otherwise.
- A local association wins, including its unreadable/invalid error state. Missing means no association, not a broken file.
- Use supplied timestamps only. Music's text property does not establish access to its catalog synchronized-lyrics UI.
- Keep provider bytes in memory. Do not log, persist, export or redistribute them. Local association storage stays unchanged.
- Retain the user's deferred physical/native acceptance; do not launch the delivered app or read the real Music library for automated tests.

## Review focus

Five failures must have targeted tests: replacement of a running player; an import completing after a provider read starts; a chosen but unreadable local file; provider plaintext or malformed/oversized timestamps; and late results after the final panel closes or the feature is disabled.

## 1. Scoped provider capability

**Files:** `apps/desktop/internal/platform/media.go`, `darwin_media.go/m/h`, focused provider tests and the existing `testdata/media-events` fixture.

**Produces:**

```go
type MediaProviderLyrics struct {
    Scope MediaScope
    Data []byte // memory only; never serialize
    Status string
    Reason string
}
type MediaProviderLyricsSource interface {
    ReadMediaProviderLyricsGuarded(context.Context, MediaScope, func() error) (MediaProviderLyrics, error)
}
```

- [x] Add declaration-only types, then failing behavior tests for exact scope, gate serialization, final guard, retirement, bounded replies and unsupported providers.
- [x] Implement the optional capability and Music-only `pLyr` getter under `pTrk`, using the existing PID-addressed noninteractive sender. Read `pPIS` and process identity before/after. Accept text descriptors only.
- [x] Return `ready`, `missing`, `unsupported` or `unavailable`; no bytes on failure. App parsing alone decides whether ready text is synchronized.
- [x] Run focused Go race tests and controlled native fixture tests. Preserve commands and red/green evidence. No real-player acceptance claim.

## 2. Local-first App integration

**Files:** `apps/desktop/app_media.go`, `app_media_assets.go`, new `app_media_lyrics.go`, `app_media_provider_lyrics_test.go` and focused edge tests. Lyric selection/read ownership is separated from artwork and RPC handlers to keep each flow readable.

**Consumes:** the optional capability above. **Produces:** `MediaLyricsView.Source` (`local`, `music` or empty), existing cues/status/reason/document ID and existing RPCs.

- [x] Add failing tests using the actual App/controller/timeline and a fake external lyric read. A ready provider reply containing `[00:00.00]First\n[00:02.00]Second` must publish two cues and source `music`; local content must win without calling the provider.
- [x] Cover plain/empty/error/mismatched-scope replies, no local adapter, explicit reload, no repeat read on position samples, and late results after import/track change/disable/final subscriber removal.
- [x] Read the local source first. Only its error-free missing result permits provider fallback. Parse with unchanged `media.ParseLRC`; publish no cues for unsynchronized text.
- [x] At import admission retire the matching pending lyric read while preserving ready content. Suppress matching reads until the chooser drains; successful import reloads local content, cancelled/failed import resumes any suspended missing content. Preserve accepted chooser lifetime across hover close.
- [x] Extend Reload to the optional provider source, keeping Remove/offset tied to local associations. Recheck asset identity, context and lyric revision at publication.
- [x] Serialize Remove/offset through a per-provider mutation owner. Matching track reads remain suspended until storage work drains, including across asset replacement; Reload and same-scope import report busy. Preserve ready content on failure and resume suspended missing content.
- [x] Run all focused App media race regressions, including existing import ownership tests.

## 3. Panel and disclosure

**Files:** `frontend/src/dock/MediaPanel.tsx` and tests, `frontend/src/lib/types.ts`, `i18n.ts`, generated models, `docs/data-handling.md` and media roadmap/report.

- [x] Test source attribution, provider Reload, retained Import, absence of local-only controls for provider content, and same-track source/document replacement resetting manual lyric scrolling.
- [x] Add optional frontend `lyrics.source` discriminator for compatibility with existing fixtures. Nonempty `documentID` remains the authority for Replace/Remove/offset.
- [x] Translate provider empty/error states in English, Portuguese and Spanish. Explain conditional Music timestamps and the local-file override without promising universal availability or control over Music's networking.
- [x] Regenerate bindings under root ownership after the model freezes; run focused frontend tests and TypeScript.

## 4. Integration

- [x] Review implementation against the exact source/admission/lifetime constraints, then address confirmed findings with regressions.
- [x] Run Go race/coverage and lint, frontend unit/TypeScript/lint/build, and relevant Chromium/WebKit media checks. Record evidence and limits; broader E04–E06 native acceptance stays open.
Delivery follows verified integration: commit and update the existing draft PR, then create a local build without launching it or publishing a release. Its exact commit, checksums and build outcome are recorded separately in the local build receipt.
