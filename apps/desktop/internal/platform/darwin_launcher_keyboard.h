#pragma once
#include <stdint.h>
typedef struct {
 uint64_t epoch, session, revision, admission;
 int enabled;
 char display[128];
} OTLauncherKeyboardPolicy;
int ot_launcher_keyboard_set(uint64_t, OTLauncherKeyboardPolicy, uintptr_t);
int ot_launcher_keyboard_valid(uint64_t, OTLauncherKeyboardPolicy);
int ot_go_launcher_keyboard_current(uintptr_t);
#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
BOOL OTLauncherKeyboardCanKey(uint64_t);
BOOL OTLauncherKeyboardEvent(uint64_t,NSEvent*);
void OTLauncherKeyboardRetire(uint64_t,BOOL);
void OTLauncherKeyboardResigned(uint64_t);
NSWindow *OTLauncherKeyboardPanel(uint64_t,const char*);
NSView *OTLauncherKeyboardContent(uint64_t);
#endif
