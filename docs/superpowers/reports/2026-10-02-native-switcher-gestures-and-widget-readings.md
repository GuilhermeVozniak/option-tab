# Native switcher gestures and widget readings

B08 now has a native input route for both switcher modes. G06 now includes
trusted built-in widget readings, action choices and action errors. This is an
implementation checkpoint on `feat/dockdoor-parity`, not a feature release or
completion of the retained roadmap.

## Switcher behavior

In **Settings → Controls**, select the window or app switcher and choose
**Swipe up** and **Swipe down** actions. Each mode retains its own choices:
none, close, minimize/restore, fullscreen toggle, hide app and quit app.
Saved choices work without opening Settings after startup.

The native local event monitor attaches to the existing overlay window. It
uses precise gesture phases and clipped rendered window-card regions; ordinary
coarse wheels and mouse drags do not become swipe actions. Toolbar, app rail,
embedded controls, scrollbars, offscreen cards, entrance animation and dismissal
frames are excluded. Small margins around controls account for browser hit-test
rounding. macOS precision/phase information cannot prove exact finger count or
exclude every precision mouse.

Each admitted gesture retains the original window, PID and process-start
identity. Action dispatch checks the current presentation and native gesture
token again after native preparation. Minimize/fullscreen preserve toggle
semantics; close and quit preserve normal application save handling. One gesture
can dispatch at most one action; momentum does not repeat it. Permission,
unsupported-action and other failures appear as localized current-presentation
feedback.

Review reproduced and corrected stale geometry during failed window lookup,
overlapping publication rollback, late errors after retirement, Settings-dependent
startup, animation hit areas, native occlusion recovery and a shared wheel-worker
shutdown/first-policy drain race. Geometry preparation uses numeric identity
capture outside the native publication lock; expensive AX admission is reserved
for the chosen action. Settings, hide, preferences, inactive sessions, host
changes and shutdown retire old gesture authority.

## Widget behavior

Battery charging/power source, network connectivity/category, audio mute state,
output chooser labels and action failures now render in English, Brazilian
Portuguese and Spanish. Host-issued presentation keys are separate from provider
values. Exact built-in ID/digest checks prevent community literals from being
rewritten. Real device names remain literal; unnamed-device fallback labels are
identified separately.

Language changes preserve provider subscriptions, package digests, grants and
admitted action tokens. Clock format choices are translated only for the trusted
built-in clock. The renderer also accepts the actual JSON omission of empty
ready text and unavailable optional readings/actions, so one missing value or
optional grant no longer hides an otherwise readable widget.

## Verification

- Full repository unit run: 542 desktop frontend, 7 shared and 4 website tests,
  with no cached tasks. All 25 Go packages passed race/coverage checks.
- Browser coverage: 113 existing desktop Chromium, 10 WebKit layout, 4 website
  Chromium and 22 focused switcher Chromium/WebKit checks; workers two, retries
  zero. The focused geometry evidence is reused only across subsequent
  presentation-string changes; final route/localization regressions cover those
  changes separately.
- Repository lint, TypeScript and golangci-lint passed. Existing compiler API,
  formatter-configuration and React test `act` warnings are nonfatal and retained
  in the logs.
- Sixteen focused App gesture tests cover all saved actions, both modes,
  retirement, delayed preparation/validation/publication and source attachment.
  Nine focused native/transport tests include an actual owned hidden NSWindow
  monitor attach, duplicate-host refusal, close notification, content replacement
  and terminal mailbox drain. Property/action seams cover event classification
  and guarded pointer mutations without acting on user windows.
- The separate [capture stress](2026-10-02-capture-lifecycle-stress.md) completed
  93 native cycles with all 372 source lifetimes drained and a peak of four.

Local receipts and source hashes are under
`.superpowers/sdd/feature-acceptance-2026-10-02/verification/`,
`.superpowers/sdd/b08-native-gestures-2026-10-02/`,
`.superpowers/sdd/widget-localization-2026-10-02/` and
`.superpowers/sdd/wheel-transport-close-2026-10-02/`.

## Remaining acceptance and implementation

Physical trackpad/precision-mouse behavior and actual packaged Wails gesture
delivery remain unverified. Hidden native fixtures and browser geometry tests
do not establish those outcomes. B08 stays unchecked pending that acceptance;
the new source route is available when native attachment succeeds.

H15 production pinch/swipe/letter input, automatic native Dock relocation,
Spotify seek runtime units and permitted provider-supplied lyrics remain
separate retained work. Packaged media/folder/automation flows, display/Space/
permission recovery, native materials, supported-OS coverage and signed universal
distribution retain their earlier acceptance limits. Sibling-product exclusions
remain unchanged.
