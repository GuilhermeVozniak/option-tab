# Focused-app launcher profiles

Implemented H03 on the existing opt-in replacement Dock. Ordered rules match an exact application bundle ID and select a profile globally or for a particular display binding. An unmatched or unavailable focused app uses the binding's base profile. A rule cannot create a Dock on an unbound display.

The native observer validates matching frontmost PID, process start and bundle identity before and after its environment read, and uses app-activation notifications to refresh its existing bounded observation. Uncertain focus clears only focus evidence. Effective-profile changes retire old interaction admission immediately, including coalesced A→B→A changes; returning to a profile creates a fresh session. Other displays and unchanged effective profiles remain independent.

Preferences supports ordered, enabled rules, exact bundle entry, bounded running-app suggestions, profile/display scope, reordering and removal. Suggestions expose names and bundle IDs only and are queried during the preferences lifetime. Deleting a profile reassigns binding and rule references together; deleting a display binding removes its scoped rules with a visible explanation. Product copy covers English, Portuguese and Spanish.

Validation at this checkpoint:

- All 23 Go packages passed race/coverage tests; repository Go lint reported zero issues.
- All JavaScript unit tests passed: 302 desktop, 7 shared and 4 site. Repository JavaScript lint passed. The production desktop TypeScript/Vite build passed during frontend integration.
- All 75 desktop Chromium workflows passed after fixing a shared test fixture that incorrectly returned empty successful settings/permissions and triggered onboarding. The fix affects the browser fake only. The focused rerun of the affected flows also passed 13/13.
- Independent integration review found no material correctness issues. Tests cover ordered/scoped rules, unknown focus, fresh sessions, blocked action retirement, two displays, configuration copies and legacy validation, reference maintenance, and preferences inventory admission.

Native tests inject focused identities and notification names. Actual focus/Spaces transitions and physical host visibility remain acceptance work; no app activation, Dock/pointer movement or native preference change was performed by this checkpoint. Existing switcher/preview native materials (B10) remain a separate implementation item; the replacement Dock material does not establish that feature.

No release is created. PR 28 remains a draft while retained replacement-Dock features and native acceptance continue.
