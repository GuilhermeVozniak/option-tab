// Checks ot_key_text_without_option against the text macOS itself gives a key
// pressed with no modifier, so the check holds on any keyboard layout.
#import <Carbon/Carbon.h>
#import <Foundation/Foundation.h>
#include "../../darwin_key_text.h"

static NSString *translated(uint16_t keycode, uint64_t flags) {
  uint16_t units[16];
  int n = ot_key_text_without_option(keycode, flags, units, 16);
  return [NSString stringWithCharacters:units length:(NSUInteger)n];
}

// A new key event takes its text from the modifiers its source has down. A
// private source has none, whatever is being held on the real keyboard.
static NSString *unmodified(uint16_t keycode) {
  CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStatePrivate);
  CGEventRef event = CGEventCreateKeyboardEvent(source, keycode, true);
  if (source != NULL) CFRelease(source);
  UniChar units[16];
  UniCharCount n = 0;
  CGEventKeyboardGetUnicodeString(event, 16, &n, units);
  CFRelease(event);
  return [NSString stringWithCharacters:units length:n];
}

int main(void) {
  @autoreleasepool {
    uint16_t units[16];
    if (ot_key_text_without_option(kVK_ANSI_T, 0, units, 16) != 0) {
      fprintf(stderr, "translated a key before any layout was read\n");
      return 1;
    }
    ot_key_layout_watch();
    ot_key_layout_watch(); // a second call must be harmless
    const uint16_t keys[] = {kVK_ANSI_T, kVK_ANSI_U, kVK_ANSI_N, kVK_ANSI_E, kVK_ANSI_1, kVK_Space};
    for (size_t i = 0; i < sizeof(keys) / sizeof(keys[0]); i++) {
      NSString *plain = unmodified(keys[i]);
      if (plain.length == 0) {
        fprintf(stderr, "key %u types nothing unmodified\n", keys[i]);
        return 1;
      }
      NSString *held = translated(keys[i], kCGEventFlagMaskAlternate);
      if (![held isEqualToString:plain]) {
        fprintf(stderr, "key %u with Option held: got '%s', want '%s'\n", keys[i], held.UTF8String,
                plain.UTF8String);
        return 1;
      }
      NSString *shifted = translated(keys[i], kCGEventFlagMaskAlternate | kCGEventFlagMaskShift);
      if (shifted.length == 0) {
        fprintf(stderr, "key %u with Option+Shift held types nothing\n", keys[i]);
        return 1;
      }
    }
    // Shift still applies: a letter key gives its capital.
    NSString *capital = translated(kVK_ANSI_T, kCGEventFlagMaskAlternate | kCGEventFlagMaskShift);
    NSString *lower = unmodified(kVK_ANSI_T);
    if (![capital isEqualToString:lower.uppercaseString] && ![lower isEqualToString:lower.uppercaseString]) {
      fprintf(stderr, "Shift lost: got '%s' for '%s'\n", capital.UTF8String, lower.UTF8String);
      return 1;
    }
    if (ot_key_text_without_option(kVK_ANSI_T, 0, units, 0) != 0 ||
        ot_key_text_without_option(kVK_ANSI_T, 0, NULL, 16) != 0) {
      fprintf(stderr, "wrote into a missing or empty buffer\n");
      return 1;
    }
    printf("ok\n");
  }
  return 0;
}
