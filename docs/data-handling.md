# Data handling

This describes the current desktop source, including the new diagnostics workflow.
Option Tab does not implement an analytics or automatic diagnostic-upload service.
It does make network requests for update checks by default. Optional remote artwork
and browser links have their own transmission boundaries, described below. macOS,
your browser, music players and any scripts you run have separate data handling.

## Local files and settings

On macOS the default application directory is
`~/Library/Application Support/option-tab/`. The menu's **Debug tools** item reveals
this directory. A custom settings path changes where the adjacent stores live.

| File | Contents and retention |
| --- | --- |
| `settings.json` | Preferences, shortcuts, filters/blacklists, feature choices and selected-display information. Saved locally when settings change. Settings export copies the settings object; review it before sharing. |
| `folder-bookmarks.json` | Folder identities and macOS security-scoped bookmark data retained for explicitly granted folders. These are sensitive access/location references, not a folder-content backup. |
| `media-lyrics.json` | Per-provider/track associations with the chosen local lyric document: bookmark, file identity, document identity and timing offset. It does not store a downloaded lyrics catalogue. |
| `crash-current.log`, `crash-pending.log` | Fatal Go runtime output and the previous pending crash, under the policy described below. |

The folder and lyric stores are application-managed JSON, not encrypted vaults.
Folder Pop enumerates a selected folder locally. Requesting access opens an explicit
folder chooser; listing does not silently open one. Opening a selected entry passes
that exact revalidated entry to macOS and its associated application. The receiving
application may have its own network behavior.

Lyrics come from an explicitly chosen local timestamped LRC file. Option Tab reads
and parses its text in memory and synchronizes lines with playback position. It may
read that associated file again when its track is presented. Removing the association
removes its stored reference; it does not delete the original lyric file. There is no
lyrics search/download service or bundled third-party lyrics database.

To remove all saved folder grants or lyric associations, quit Option Tab and remove
only the corresponding store file above. Removing `settings.json` resets preferences
on the next launch. Disabling a feature alone does not erase its saved references.
These steps do not revoke macOS privacy permissions, erase original files, or remove
copies you exported/shared. Manage macOS permissions separately in System Settings.

## Window previews and local automation

Window capture uses macOS Screen Recording permission. Preview images and their
window/process identity metadata are cached in process memory, not written as a
screenshot history by Option Tab. Cached images can remain until replaced, cleared
or the process ends; closing a panel is not a promise of immediate cache erasure.
Background thumbnail capture is off by default and requires the separate
`CaptureInBackground` option. Enabling it can keep the screen-recording indicator on
while the switcher is hidden. Visible previews can capture without that background
option.

The local AppleEvents automation interface lets a caller request app/window metadata,
perform supported actions, and show previews. It checks local same-user sender
provenance and macOS permission/admission constraints. It is not an HTTP listener or
a generic script evaluator. An explicit cached-image query may return existing PNG
images after identity/freshness checks; that query does not start a new capture.
Metadata and image replies go to the requesting script. A script can subsequently
save or transmit them, so consider what the script requests before running it.
Diagnostics reports exclude these images and app/window identities.

## Replacement-Dock badges

Badge display is off by default for each replacement-Dock profile. When enabled
on a visible Dock, Option Tab reads available status labels from the native macOS
Dock through Accessibility. It matches exact application identities locally; it
does not read Notification Center or notification message bodies. App and macOS
support varies, and an unavailable value is not treated as a zero count.

Labels are immediately reduced to a bounded numeric count, a generic indicator,
or no badge. Raw label text is not retained, logged or sent to the interface.
Application paths and process identities remain in the native/Go observation
layer. The interface receives only its existing opaque item ID and typed badge
value. Values stay in memory; this feature writes no badge history and makes no
network request. Disabling the profile option or retiring its Dock cancels its
observation and prevents late results from returning to that Dock session.

## Music, Spotify and artwork

Media previews, each player adapter, and remote artwork are off by default. Enabled
Music/Spotify adapters communicate with the installed running player through fixed
AppleEvents for metadata and supported playback controls; Option Tab does not launch
a player just to poll it. Explicit permission requests may show macOS Automation
consent. Normal polling does not silently request that consent.

Music artwork is obtained locally from Music. Spotify remote artwork requires the
separate remote-artwork setting. When enabled, Option Tab makes HTTPS requests to
public artwork URLs supplied by Spotify, which may identify the requested track or
cover. The remote host receives the URL and ordinary connection/request metadata,
including the connecting IP address. The adapter validates destinations and redirects,
rejects private/local endpoints, limits input to 5 MiB and bounds request duration.
It does not send a window screenshot, lyrics file, settings export or Spotify account
credential in that artwork request.

Normalized artwork is cached in memory with a 20 MiB cache budget. Disabling remote
artwork cancels ongoing work and prevents late results from appearing; it cannot
recall a request already received by the remote host. Player/network activity outside
Option Tab is not controlled by this setting.

## Updates and external links

The default update policy is **Check**, with a request about 10 seconds after launch
and then every 24 hours to:

`https://api.github.com/repos/GuilhermeVozniak/option-tab/releases/latest`

The request does not attach settings, activity logs or window data. GitHub still
receives ordinary network/request metadata. **Off** disables the scheduled checks;
a manual **Check for updates** still makes a request. **Check** announces an available
release without automatically installing it. **Auto** can download, install and
relaunch after a scheduled check. A manual check never auto-installs by itself.

Installation downloads the selected GitHub release asset, following its download
redirects, into a temporary DMG; the installer attempts to remove that file afterward.
See [distribution](distribution.md) for the exact architecture selection contract and
[Homebrew](homebrew.md) for the separate installation route.

Project, feedback, support and release links open your browser. The destination
receives the navigation request; browser history, logged-in accounts and subsequent
sharing are governed by that browser/service. This also applies when visiting the
project website, hosted separately from the desktop application.

## Crash reports: browser navigation already sends the text

Crash reporting defaults to **Ask**. At launch, a nonempty current runtime crash log
is promoted to `crash-pending.log`, and a new current file is created/truncated for
fatal Go runtime output. A later crash can replace the pending one. The files are
not the sanitized diagnostics report and can contain stack traces, paths, panic
messages or other sensitive text. There is no automatic crash upload. In the current
implementation, the setting labeled **Always send** still uses the report/dismiss
flow; it does not add an automatic upload path.

**Report Crash opens a GitHub new-issue URL whose query contains up to 3,000 bytes of
raw crash text, plus a truncation marker when needed. That text is transmitted to
GitHub when the browser navigates, before you press Submit.** Reviewing the form
before publishing an issue does not undo this initial transfer. The URL can also
appear in browser history. Do not click Report if you do not want that transfer.

**Dismiss** deletes the pending crash file. Choosing **Never** prevents setup of new
crash capture at the next launch and dismisses pending output; it is not a guarantee
that an existing current file has been erased or that already-armed capture stops
mid-run. Quit before manually deleting either named crash file. macOS-managed native
crash reports and ordinary process/stderr logs are separate. `OPTIONTAB_DEBUG` enables
additional stderr output that can contain raw errors or paths; diagnostics export
does not collect or sanitize those existing logs.

## Diagnostics: reviewed local reports

Diagnostics recording starts off and is not persisted across launches. Starting a
recording begins a new in-memory session, limited to 10 minutes, 1,024 complete events
and 256 KiB of serialized event data. Oldest events are evicted at the limits and a
dropped count is retained. **Stop** retains data for review; **Clear** stops recording
and removes it. Shutdown discards the service's in-memory data.

A report contains an allowlisted version/runtime/platform snapshot, feature and
coarse permission status, enumerated events, relative elapsed times and bounded
numeric counts/durations. Those status facts may still be private to you. It excludes
raw settings/logs/errors/crash text; app/window names and identities; paths/bookmarks;
keyboard or clipboard contents; screenshots/artwork; and track metadata or lyrics.
It does not read crash bodies or collect arbitrary event payloads.

Review creates one immutable JSON snapshot, at most 512 KiB, valid for five minutes.
Saving uses those reviewed bytes rather than rebuilding the report. Only one export
can be pending. Session inactivity invalidates the review and cancels pending work
but retains recording data; Clear additionally removes that data. Canceling the
Save dialog itself leaves the same reviewed snapshot available until it expires.

The native Save operation writes a new file at your chosen destination. Existing
files are refused, even if a system dialog offers replacement: choose a new filename.
There is no automatic upload, browser opening, clipboard copy or persistent export
path history in this workflow. An export that has already committed remains saved
if cancellation arrives afterward. Saved reports are your files; stopping recording,
clearing diagnostics or quitting does not delete them or any copies you share.
