#ifndef OT_MEDIA_PANEL_H
#define OT_MEDIA_PANEL_H
#include <stdint.h>
uint64_t ot_media_panel_create(void *,uint64_t);
char *ot_media_panel_next(uint64_t);
void ot_media_panel_forget(uint64_t);
#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
void OTMediaAttach(uint64_t,uint64_t,NSPanel *);
NSRect OTMediaPrepare(uint64_t,NSRect);
BOOL OTMediaHeaderEvent(uint64_t,NSEvent *);
void OTMediaHide(uint64_t);
void OTMediaShown(uint64_t);
void OTMediaDestroy(uint64_t,BOOL);
#endif
#endif
