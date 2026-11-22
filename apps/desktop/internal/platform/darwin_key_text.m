// The text a key types once the switcher's own Option key is set aside.
//
// While Option is held to keep the switcher open, every letter arrives from
// the event tap as its Option character ("†" for T), which matches no window.
// The event cannot be asked for its unmodified text, so the key is translated
// again from the keyboard layout without Option.

#import <Carbon/Carbon.h>
#import <CoreGraphics/CoreGraphics.h>
#include <os/lock.h>
#include "darwin_key_text.h"

static os_unfair_lock gLayoutLock = OS_UNFAIR_LOCK_INIT;
static CFDataRef gLayoutData; // guarded by gLayoutLock
static UInt32 gKeyboardType;  // guarded by gLayoutLock

void ot_key_layout_refresh(void) {
  CFDataRef data = NULL;
  TISInputSourceRef source = TISCopyCurrentKeyboardLayoutInputSource();
  if (source != NULL) {
    CFDataRef layout = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData);
    if (layout != NULL) data = CFRetain(layout);
    CFRelease(source);
  }
  UInt32 type = LMGetKbdType();
  os_unfair_lock_lock(&gLayoutLock);
  CFDataRef old = gLayoutData;
  gLayoutData = data;
  gKeyboardType = type;
  os_unfair_lock_unlock(&gLayoutLock);
  if (old != NULL) CFRelease(old);
}

static void layoutChanged(CFNotificationCenterRef center, void *observer, CFNotificationName name,
                          const void *object, CFDictionaryRef info) {
  (void)center;
  (void)observer;
  (void)name;
  (void)object;
  (void)info;
  ot_key_layout_refresh();
}

void ot_key_layout_watch(void) {
  static int watching;
  ot_key_layout_refresh();
  if (watching) return;
  watching = 1;
  // Delivered on the registering thread's run loop, which is the main one.
  CFNotificationCenterAddObserver(CFNotificationCenterGetDistributedCenter(), NULL, layoutChanged,
                                  kTISNotifySelectedKeyboardInputSourceChanged, NULL,
                                  CFNotificationSuspensionBehaviorDeliverImmediately);
}

int ot_key_text_without_option(uint16_t keycode, uint64_t flags, uint16_t *out, int capacity) {
  if (out == NULL || capacity <= 0) return 0;
  os_unfair_lock_lock(&gLayoutLock);
  CFDataRef data = gLayoutData != NULL ? CFRetain(gLayoutData) : NULL;
  UInt32 type = gKeyboardType;
  os_unfair_lock_unlock(&gLayoutLock);
  if (data == NULL) return 0;

  // UCKeyTranslate takes Carbon modifier bits shifted down one byte.
  UInt32 modifiers = 0;
  if (flags & kCGEventFlagMaskShift) modifiers |= (shiftKey >> 8) & 0xFF;
  if (flags & kCGEventFlagMaskAlphaShift) modifiers |= (alphaLock >> 8) & 0xFF;
  UInt32 deadKeys = 0;
  UniCharCount length = 0;
  OSStatus status =
      UCKeyTranslate((const UCKeyboardLayout *)CFDataGetBytePtr(data), keycode, kUCKeyActionDown,
                     modifiers, type, kUCKeyTranslateNoDeadKeysMask, &deadKeys,
                     (UniCharCount)capacity, &length, out);
  CFRelease(data);
  if (status != noErr || length > (UniCharCount)capacity) return 0;
  return (int)length;
}
