#import "../../darwin_audio_output.m"
int goAudioOutputFinalGuard(uintptr_t token) { abort(); }
int main(void) {
  @autoreleasepool {
    __block AudioDeviceID id = 10;
    __block BOOL allowed = YES, settable = YES;
    __block int writes = 0;
    AudioDeviceID (^resolve)(void) = ^AudioDeviceID {
      return id;
    };
    BOOL (^canSet)(void) = ^BOOL {
      return settable;
    };
    BOOL (^guard)(void) = ^BOOL {
      return allowed;
    };
    BOOL (^write)(AudioDeviceID) = ^BOOL(AudioDeviceID selected) {
      writes++;
      return selected == 10;
    };
    BOOL (^readback)(void) = ^BOOL {
      return YES;
    };
    NSCAssert(audioSelectPrepared(resolve, canSet, guard, write, readback) ==
                      0 &&
                  writes == 1,
              @"valid selection failed");
    for (int mode = 0; mode < 3; mode++) {
      id = 10;
      allowed = settable = YES;
      writes = 0;
      int status = audioSelectPrepared(
          resolve, canSet,
          ^BOOL {
            if (mode == 0)
              id = 20;
            if (mode == 1)
              settable = NO;
            if (mode == 2)
              allowed = NO;
            return allowed;
          },
          write, readback);
      NSCAssert(status != 0 && writes == 0,
                @"UID/device replacement, settable change or cancelled guard "
                @"dispatched");
    }
    id = 10;
    settable = allowed = YES;
    writes = 0;
    NSCAssert(audioSelectPrepared(
                  resolve, canSet, guard,
                  ^BOOL(AudioDeviceID target) {
                    return NO;
                  },
                  readback) == 5,
              @"write failure hidden");
    NSCAssert(audioSelectPrepared(resolve, canSet, guard, write,
                                  ^BOOL {
                                    return NO;
                                  }) == 6,
              @"readback mismatch hidden");
    puts("PASS injected native audio selection: exact ID re-resolution, "
         "cancellation/settable guards, rejected write and readback; no HAL "
         "call/device query/action");
  }
  return 0;
}
