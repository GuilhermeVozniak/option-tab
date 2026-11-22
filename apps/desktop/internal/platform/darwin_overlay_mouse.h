#ifndef OT_OVERLAY_MOUSE_H
#define OT_OVERLAY_MOUSE_H

// ot_overlay_accept_first_click makes the web view inside window (an
// NSWindow *) act on a click made while another app is active, instead of
// spending that click on activating this app. Main thread only. Calling it
// again for the same window does nothing.
void ot_overlay_accept_first_click(void *window);

#endif
