//go:build darwin
#import "darwin_audio_output.h"
#import <CoreAudio/CoreAudio.h>
#import <Foundation/Foundation.h>
#import <stdatomic.h>
extern int goAudioOutputFinalGuard(uintptr_t);
static AudioObjectPropertyAddress
audioAddress(AudioObjectPropertySelector selector,
             AudioObjectPropertyScope scope) {
  return (AudioObjectPropertyAddress){selector, scope,
                                      kAudioObjectPropertyElementMain};
}
static BOOL audioValue(AudioObjectID id, AudioObjectPropertySelector selector,
                       AudioObjectPropertyScope scope, void *out,
                       UInt32 expected) {
  AudioObjectPropertyAddress a = audioAddress(selector, scope);
  UInt32 size = expected;
  return AudioObjectHasProperty(id, &a) &&
         AudioObjectGetPropertyData(id, &a, 0, NULL, &size, out) == noErr &&
         size == expected;
}
static NSString *audioString(AudioObjectID id,
                             AudioObjectPropertySelector selector) {
  CFStringRef value = NULL;
  if (!audioValue(id, selector, kAudioObjectPropertyScopeGlobal, &value,
                  sizeof(value)))
    return nil;
  NSString *result = CFBridgingRelease(value);
  return [result isKindOfClass:NSString.class] && [result length] > 0 &&
                 [result length] <= 1024
             ? result
             : nil;
}
static int audioChannels(AudioObjectID id) {
  AudioObjectPropertyAddress a = audioAddress(
      kAudioDevicePropertyStreamConfiguration, kAudioDevicePropertyScopeOutput);
  UInt32 size = 0;
  if (AudioObjectGetPropertyDataSize(id, &a, 0, NULL, &size) != noErr ||
      size < offsetof(AudioBufferList, mBuffers) || size > 65536)
    return -1;
  AudioBufferList *list = calloc(1, size);
  UInt32 actual = size;
  int channels = -1;
  if (AudioObjectGetPropertyData(id, &a, 0, NULL, &actual, list) == noErr &&
      actual == size && list->mNumberBuffers <= 64 &&
      offsetof(AudioBufferList, mBuffers) +
              list->mNumberBuffers * sizeof(AudioBuffer) <=
          size) {
    channels = 0;
    for (UInt32 i = 0; i < list->mNumberBuffers; i++) {
      if (list->mBuffers[i].mNumberChannels > 1024) {
        channels = -1;
        break;
      }
      channels += list->mBuffers[i].mNumberChannels;
    }
    if (channels > 1024)
      channels = -1;
  }
  free(list);
  return channels;
}
static NSArray *audioDevices(void) {
  AudioObjectPropertyAddress a = audioAddress(kAudioHardwarePropertyDevices,
                                              kAudioObjectPropertyScopeGlobal);
  UInt32 size = 0;
  if (AudioObjectGetPropertyDataSize(kAudioObjectSystemObject, &a, 0, NULL,
                                     &size) != noErr ||
      size > 64 * sizeof(AudioDeviceID) || size % sizeof(AudioDeviceID))
    return nil;
  AudioDeviceID ids[64] = {0};
  UInt32 actual = size;
  if (size && AudioObjectGetPropertyData(kAudioObjectSystemObject, &a, 0, NULL,
                                         &actual, ids) != noErr)
    return nil;
  if (actual != size)
    return nil;
  NSMutableArray *devices = [NSMutableArray array];
  NSMutableSet *seen = [NSMutableSet set];
  for (UInt32 i = 0; i < size / sizeof(AudioDeviceID); i++) {
    UInt32 alive = 0;
    if (!audioValue(ids[i], kAudioDevicePropertyDeviceIsAlive,
                    kAudioObjectPropertyScopeGlobal, &alive, sizeof(alive)))
      return nil;
    if (!alive)
      continue;
    int channels = audioChannels(ids[i]);
    if (channels < 0)
      return nil;
    if (channels == 0)
      continue;
    NSString *uid = audioString(ids[i], kAudioDevicePropertyDeviceUID),
             *name = audioString(ids[i], kAudioObjectPropertyName);
    if (!uid || !name || [seen containsObject:uid])
      return nil;
    [seen addObject:uid];
    [devices addObject:@{
      @"UID" : uid,
      @"Name" : name,
      @"Alive" : @YES,
      @"OutputChannels" : @(channels),
      @"nativeID" : @(ids[i])
    }];
  }
  return devices;
}
static AudioDeviceID audioResolve(NSString *uid) {
  NSArray *devices = audioDevices();
  for (NSDictionary *d in devices)
    if ([d[@"UID"] isEqual:uid])
      return [d[@"nativeID"] unsignedIntValue];
  return kAudioObjectUnknown;
}
static NSString *audioDefault(void) {
  AudioDeviceID id = 0;
  if (!audioValue(kAudioObjectSystemObject,
                  kAudioHardwarePropertyDefaultOutputDevice,
                  kAudioObjectPropertyScopeGlobal, &id, sizeof(id)) ||
      !id)
    return nil;
  return audioString(id, kAudioDevicePropertyDeviceUID);
}
static BOOL audioSettable(void) {
  AudioObjectPropertyAddress a =
      audioAddress(kAudioHardwarePropertyDefaultOutputDevice,
                   kAudioObjectPropertyScopeGlobal);
  Boolean settable = false;
  return AudioObjectIsPropertySettable(kAudioObjectSystemObject, &a,
                                       &settable) == noErr &&
         settable;
}
// All preparation is outside a listener/source lock. Injected blocks exercise
// the same final UID/ID/guard/write/readback pipeline without accessing actual
// HAL.
static int audioSelectPrepared(AudioDeviceID (^resolve)(void),
                               BOOL (^settable)(void), BOOL (^guard)(void),
                               BOOL (^write)(AudioDeviceID),
                               BOOL (^readback)(void)) {
  AudioDeviceID target = resolve();
  if (!target)
    return 2;
  if (!settable())
    return 3;
  if (!guard())
    return 4;
  if (resolve() != target || !settable())
    return 2;
  if (!guard())
    return 4;
  if (!write(target))
    return 5;
  return readback() ? 0 : 6;
}
int ot_audio_output_select(const char *text, uintptr_t guard) {
  @autoreleasepool {
    NSString *uid = text ? [NSString stringWithUTF8String:text] : nil;
    if (!uid.length || uid.length > 1024)
      return 1;
    return audioSelectPrepared(
        ^AudioDeviceID {
          return audioResolve(uid);
        },
        ^BOOL {
          return audioSettable();
        },
        ^BOOL {
          return goAudioOutputFinalGuard(guard) != 0;
        },
        ^BOOL(AudioDeviceID target) {
          AudioObjectPropertyAddress a =
              audioAddress(kAudioHardwarePropertyDefaultOutputDevice,
                           kAudioObjectPropertyScopeGlobal);
          return AudioObjectSetPropertyData(kAudioObjectSystemObject, &a, 0,
                                            NULL, sizeof(target),
                                            &target) == noErr;
        },
        ^BOOL {
          return [audioDefault() isEqual:uid];
        });
  }
}
@interface OTAudioOutputOwner : NSObject {
@public
  atomic_bool dirty;
}
@property dispatch_queue_t queue;
@property AudioObjectPropertyListenerBlock listener;
@property NSMutableArray *entries;
@property AudioDeviceID selected;
@property BOOL failed;
@end
@implementation OTAudioOutputOwner
@end
static BOOL audioListen(OTAudioOutputOwner *owner, AudioObjectID id,
                        AudioObjectPropertySelector selector,
                        AudioObjectPropertyScope scope, BOOL optional) {
  AudioObjectPropertyAddress a = audioAddress(selector, scope);
  if (!AudioObjectHasProperty(id, &a))
    return optional;
  if (AudioObjectAddPropertyListenerBlock(id, &a, owner.queue,
                                          owner.listener) != noErr)
    return NO;
  [owner.entries addObject:@[ @(id), @(selector), @(scope) ]];
  return YES;
}
static void audioUnlisten(OTAudioOutputOwner *owner, BOOL all) {
  for (NSArray *entry in [owner.entries copy]) {
    if (!all && [entry[0] unsignedIntValue] == kAudioObjectSystemObject)
      continue;
    AudioObjectPropertyAddress a =
        audioAddress([entry[1] unsignedIntValue], [entry[2] unsignedIntValue]);
    AudioObjectRemovePropertyListenerBlock([entry[0] unsignedIntValue], &a,
                                           owner.queue, owner.listener);
    [owner.entries removeObject:entry];
  }
}
void *ot_audio_output_start(void) {
  @autoreleasepool {
    OTAudioOutputOwner *owner = [OTAudioOutputOwner new];
    atomic_init(&owner->dirty, true);
    owner.queue = dispatch_queue_create("org.optiontab.audio-output",
                                        DISPATCH_QUEUE_SERIAL);
    owner.entries = [NSMutableArray array];
    __weak OTAudioOutputOwner *weak = owner;
    owner.listener =
        ^(UInt32 count, const AudioObjectPropertyAddress *addresses) {
          OTAudioOutputOwner *strong = weak;
          if (strong)
            atomic_store(&strong->dirty, true);
        };
    __block BOOL ok = NO;
    dispatch_sync(owner.queue, ^{
      ok = audioListen(owner, kAudioObjectSystemObject,
                       kAudioHardwarePropertyDevices,
                       kAudioObjectPropertyScopeGlobal, NO) &&
           audioListen(owner, kAudioObjectSystemObject,
                       kAudioHardwarePropertyDefaultOutputDevice,
                       kAudioObjectPropertyScopeGlobal, NO);
      if (!ok)
        audioUnlisten(owner, YES);
    });
    return ok ? (__bridge_retained void *)owner : NULL;
  }
}
char *ot_audio_output_read(void *pointer) {
  @autoreleasepool {
    OTAudioOutputOwner *owner = (__bridge OTAudioOutputOwner *)pointer;
    NSArray *devices = audioDevices();
    NSString *uid = audioDefault();
    NSMutableDictionary *result = [@{
      @"Status" : @"unavailable",
      @"Reason" : @"inventoryUnavailable",
      @"Devices" : @[]
    } mutableCopy];
    if (devices) {
      NSMutableArray *public = [NSMutableArray array];
      AudioDeviceID selected = 0;
      for (NSDictionary *d in devices) {
        NSMutableDictionary *item = [d mutableCopy];
        [item removeObjectForKey:@"nativeID"];
        [public addObject:item];
        if ([d[@"UID"] isEqual:uid])
          selected = [d[@"nativeID"] unsignedIntValue];
      }
      result[@"Devices"] = public;
      result[@"DefaultUID"] = selected ? uid : @"";
      result[@"Status"] = selected ? @"ready" : @"unavailable";
      result[@"Reason"] = selected ? @"" : @"defaultUnavailable";
      dispatch_sync(owner.queue, ^{
        if (owner.selected != selected) {
          audioUnlisten(owner, NO);
          owner.selected = selected;
          owner.failed = NO;
          if (selected)
            owner.failed = !(
                audioListen(owner, selected, kAudioDevicePropertyDeviceIsAlive,
                            kAudioObjectPropertyScopeGlobal, NO) &&
                audioListen(owner, selected, kAudioDevicePropertyVolumeScalar,
                            kAudioDevicePropertyScopeOutput, YES) &&
                audioListen(owner, selected, kAudioDevicePropertyMute,
                            kAudioDevicePropertyScopeOutput, YES));
        }
      });
      if (owner.failed) {
        result[@"Status"] = @"unavailable";
        result[@"Reason"] = @"listenerUnavailable";
      }
      if (selected) {
        Float32 volume = 0;
        UInt32 muted = 0;
        if (audioValue(selected, kAudioDevicePropertyVolumeScalar,
                       kAudioDevicePropertyScopeOutput, &volume,
                       sizeof(volume)) &&
            isfinite(volume) && volume >= 0 && volume <= 1)
          result[@"Volume"] = @(volume);
        if (audioValue(selected, kAudioDevicePropertyMute,
                       kAudioDevicePropertyScopeOutput, &muted,
                       sizeof(muted)) &&
            muted <= 1)
          result[@"Muted"] = muted ? @YES : @NO;
        if (![audioString(selected, kAudioDevicePropertyDeviceUID)
                isEqual:uid] ||
            ![audioDefault() isEqual:uid]) {
          result[@"Status"] = @"unavailable";
          result[@"Reason"] = @"deviceChanged";
          [result removeObjectForKey:@"Volume"];
          [result removeObjectForKey:@"Muted"];
        }
      }
    }
    NSData *data = [NSJSONSerialization dataWithJSONObject:result
                                                   options:0
                                                     error:NULL];
    return data ? strdup([[NSString alloc] initWithData:data
                                               encoding:NSUTF8StringEncoding]
                             .UTF8String)
                : NULL;
  }
}
int ot_audio_output_dirty(void *pointer) {
  OTAudioOutputOwner *owner = (__bridge OTAudioOutputOwner *)pointer;
  return atomic_exchange(&owner->dirty, false);
}
void ot_audio_output_close(void *pointer) {
  OTAudioOutputOwner *owner = CFBridgingRelease(pointer);
  dispatch_sync(owner.queue, ^{
    audioUnlisten(owner, YES);
  });
  dispatch_sync(owner.queue, ^{
                });
  owner.listener = nil;
}
