# Pure local automation service

`New(Deps).Handle(ctx, platform.AutomationRequest)` validates the frozen typed
request and returns either bounded UTF-8 error fields or JSON schemaVersion 1.
No event handlers, UI, capture, network, permissions or native mutation are
implemented in this package; injectable ports own those effects.

The optional Admission hook runs at request start, final guards and completion.
Nil means no additional App policy, intended for isolated fixtures. Presentation
callbacks must invoke their supplied guard after preparation before UI admission.
The guarded native performer must invoke it immediately before mutation and
reverify the supplied process/window identity. Minimize means set true; fullscreen
requires its explicit desired bool. Hide means the resolved window's owning app.
The active-window source is read once; subsequent foreground changes never retarget
that captured window.

Fresh inventories returning both values and an error are refused. Selectors match
exact Unicode name, exact bundle ID, or positive PID, with no coercion, fallback,
autolaunch or duplicate disambiguation. Resolved applications are reread after
process identity capture. Request contexts default to 5 seconds and never exceed
10 seconds, preserving a shorter caller deadline. Dependencies must honor contexts
or have their own finite bound: this service cannot forcibly interrupt a blocked
external callback or recall an already dispatched native mutation.

Success JSON uses apps/windows/active/presentation fields as appropriate. Empty
inventories are arrays. windowID, spaceID and process startSeconds are decimal
strings to preserve integer identity for JavaScript/macro clients; PID and start
microseconds are bounded numeric values. Bounds use domain's X/Y/W/H logical
coordinates. Presentation token strings are server-issued positive decimal IDs.
The service does not equate an accepted presentation with visible rendering.

Inventory replies cap at 500 entries and 4 MiB. Entries with unsupported oversized
text or beyond the byte/count budget are omitted explicitly through truncated and
omitted. Returned metadata text is JSON escaped, never interpreted.

Image queries are opt-in and read only copied existing cache data. A cached frame
must match exact window/process identity, be no older than 30 seconds and not be
future dated. At most 8 complete PNG data URLs, each at most 512 KiB encoded, fit
inside the reply budget; invalid/oversized/expired/missing/refused cache entries
have explicit per-window imageStatus. PNG headers are checked without decoding
pixels (maximum 8192 per dimension and 16,777,216 pixels). Images are never created,
refreshed or fetched by this service. Cache failures leave valid inventory metadata
available and mark images unavailable.
