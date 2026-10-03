#import <CoreAudio/CoreAudio.h>
#import <Foundation/Foundation.h>
static NSString *fixtureUID = @"output";
static int added, removed, writes, guardMode;
static BOOL failListener, huge;
static Boolean fakeHas(AudioObjectID device,
                       const AudioObjectPropertyAddress *a) {
  return a->mSelector != kAudioDevicePropertyVolumeScalar &&
         a->mSelector != kAudioDevicePropertyMute;
}
static OSStatus fakeSize(AudioObjectID device,
                         const AudioObjectPropertyAddress *a, UInt32 q,
                         const void *v, UInt32 *size) {
  *size = a->mSelector == kAudioHardwarePropertyDevices
              ? (huge ? 100000 : 2 * sizeof(AudioDeviceID))
              : sizeof(AudioBufferList);
  return noErr;
}
static OSStatus fakeGet(AudioObjectID device,
                        const AudioObjectPropertyAddress *a, UInt32 q,
                        const void *v, UInt32 *size, void *out) {
  switch (a->mSelector) {
  case kAudioHardwarePropertyDevices: {
    AudioDeviceID values[] = {10, 20};
    memcpy(out, values, sizeof(values));
    *size = sizeof(values);
    return noErr;
  }
  case kAudioHardwarePropertyDefaultOutputDevice:
    *(AudioDeviceID *)out = 10;
    *size = sizeof(AudioDeviceID);
    return noErr;
  case kAudioDevicePropertyDeviceIsAlive:
    *(UInt32 *)out = 1;
    *size = sizeof(UInt32);
    return noErr;
  case kAudioDevicePropertyStreamConfiguration: {
    AudioBufferList *list = out;
    list->mNumberBuffers = 1;
    list->mBuffers[0].mNumberChannels = device == 10 ? 2 : 0;
    *size = sizeof(AudioBufferList);
    return noErr;
  }
  case kAudioDevicePropertyDeviceUID:
    *(CFStringRef *)out =
        CFBridgingRetain(device == 10 ? fixtureUID : @"input");
    *size = sizeof(CFStringRef);
    return noErr;
  case kAudioObjectPropertyName:
    *(CFStringRef *)out = CFBridgingRetain(@"Fixture device");
    *size = sizeof(CFStringRef);
    return noErr;
  default:
    abort();
  }
}
static OSStatus fakeSettable(AudioObjectID device,
                             const AudioObjectPropertyAddress *a,
                             Boolean *out) {
  NSCAssert(a->mSelector == kAudioHardwarePropertyDefaultOutputDevice,
            @"wrong control");
  *out = true;
  return noErr;
}
static OSStatus fakeSet(AudioObjectID device,
                        const AudioObjectPropertyAddress *a, UInt32 q,
                        const void *v, UInt32 size, const void *data) {
  NSCAssert(device == kAudioObjectSystemObject &&
                a->mSelector == kAudioHardwarePropertyDefaultOutputDevice &&
                *(AudioDeviceID *)data == 10,
            @"wrong mutation");
  writes++;
  return noErr;
}
static OSStatus fakeAdd(AudioObjectID d, const AudioObjectPropertyAddress *a,
                        dispatch_queue_t queue,
                        AudioObjectPropertyListenerBlock block) {
  if (failListener && added % 2)
    return -1;
  added++;
  return noErr;
}
static OSStatus fakeRemove(AudioObjectID d, const AudioObjectPropertyAddress *a,
                           dispatch_queue_t queue,
                           AudioObjectPropertyListenerBlock block) {
  removed++;
  return noErr;
}
#define AudioObjectHasProperty fakeHas
#define AudioObjectGetPropertyDataSize fakeSize
#define AudioObjectGetPropertyData fakeGet
#define AudioObjectIsPropertySettable fakeSettable
#define AudioObjectSetPropertyData fakeSet
#define AudioObjectAddPropertyListenerBlock fakeAdd
#define AudioObjectRemovePropertyListenerBlock fakeRemove
#import "../../darwin_audio_output.m"
int goAudioOutputFinalGuard(uintptr_t token) {
  if (guardMode == 1)
    fixtureUID = @"reused";
  return guardMode != 2;
}
int main(void) {
  @autoreleasepool {
    void *p = ot_audio_output_start();
    NSCAssert(p && added == 2, @"listener startup");
    char *raw = ot_audio_output_read(p);
    NSDictionary *state = [NSJSONSerialization
        JSONObjectWithData:[[NSString stringWithUTF8String:raw]
                               dataUsingEncoding:NSUTF8StringEncoding]
                   options:0
                     error:NULL];
    free(raw);
    NSCAssert([state[@"Status"] isEqual:@"ready"] &&
                  [state[@"Devices"] count] == 1 &&
                  [state[@"DefaultUID"] isEqual:@"output"],
              @"output-only inventory wrong");
    NSCAssert(!state[@"Volume"] && !state[@"Muted"],
              @"unsupported values fabricated");
    NSCAssert(ot_audio_output_select("output", 1) == 0 && writes == 1,
              @"fixed actual HAL pipeline refused");
    writes = 0;
    guardMode = 1;
    NSCAssert(ot_audio_output_select("output", 1) != 0 && writes == 0,
              @"same numeric ID reused for other UID dispatched");
    fixtureUID = @"output";
    guardMode = 2;
    NSCAssert(ot_audio_output_select("output", 1) != 0 && writes == 0,
              @"guard cancellation dispatched");
    OTAudioOutputOwner *owner = (__bridge OTAudioOutputOwner *)p;
    dispatch_sync(owner.queue, ^{
      for (int i = 0; i < 1000; i++)
        owner.listener(0, NULL);
    });
    NSCAssert(ot_audio_output_dirty(p) && !ot_audio_output_dirty(p),
              @"listener burst did not coalesce");
    ot_audio_output_close(p);
    NSCAssert(added == removed, @"listener cleanup leaked");
    huge = YES;
    NSCAssert(!audioDevices(), @"oversized inventory accepted");
    huge = NO;
    added = removed = 0;
    failListener = YES;
    NSCAssert(!ot_audio_output_start() && added == removed,
              @"partial startup listener leak");
    puts("PASS injected HAL: output-only inventory, nullable controls, UID "
         "reuse/cancel refusal, default-output-only mutation, coalescing, "
         "joined listener cleanup and partial-start failure; no real device "
         "queries/actions");
  }
  return 0;
}
