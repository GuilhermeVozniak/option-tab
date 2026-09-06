//go:build darwin
#import "darwin_stream.h"
#import "darwin.h"
#import <ApplicationServices/ApplicationServices.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef element,
                                     CGWindowID *window);
#import <Cocoa/Cocoa.h>
#import <CoreImage/CoreImage.h>
#import <CoreMedia/CoreMedia.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
extern void goStreamFrame(uint64_t token, char *data);
extern void goStreamDone(uint64_t token);
extern void goStreamUnavailable(uint64_t token);

@interface OTWindowStream : NSObject <SCStreamOutput, SCStreamDelegate>
@property uint64_t token;
@property SCStream *stream;
@property CIContext *images;
@property BOOL stopped;
@property BOOL starting;
@property AXObserverRef observer;
@property AXUIElementRef observedWindow;
@property CFTimeInterval lastFrame;
@end
static dispatch_queue_t controlQueue(void) {
  static dispatch_queue_t q;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    q = dispatch_queue_create("com.optiontab.stream.control",
                              DISPATCH_QUEUE_SERIAL);
  });
  return q;
}
static NSMutableDictionary *sessions(void) {
  static NSMutableDictionary *d;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    d = [NSMutableDictionary new];
  });
  return d;
}
// AX notifications need a running CFRunLoop even in non-AppKit callers. One
// idle observer thread is shared by all capture sessions; no frames run here.
static CFRunLoopRef observationLoop(void) {
  static CFRunLoopRef loop;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    dispatch_semaphore_t ready = dispatch_semaphore_create(0);
    NSThread *thread = [[NSThread alloc] initWithBlock:^{
      @autoreleasepool {
        loop = CFRunLoopGetCurrent();
        CFRetain(loop);
        [[NSRunLoop currentRunLoop] addPort:[NSMachPort port]
                                    forMode:NSDefaultRunLoopMode];
        dispatch_semaphore_signal(ready);
        CFRunLoopRun();
      }
    }];
    thread.name = @"Option Tab capture lifecycle";
    [thread start];
    dispatch_semaphore_wait(ready, DISPATCH_TIME_FOREVER);
  });
  return loop;
}
static void finish(OTWindowStream *s) {
  if (s.observer) {
    CFRunLoopRemoveSource(observationLoop(),
                          AXObserverGetRunLoopSource(s.observer),
                          kCFRunLoopCommonModes);
    if (s.observedWindow)
      AXObserverRemoveNotification(s.observer, s.observedWindow,
                                   kAXUIElementDestroyedNotification);
    CFRelease(s.observer);
    s.observer = NULL;
  }
  if (s.observedWindow) {
    CFRelease(s.observedWindow);
    s.observedWindow = NULL;
  }
  [sessions() removeObjectForKey:@(s.token)];
  s.stream = nil;
  goStreamDone(s.token);
}
static void stopSession(OTWindowStream *s) {
  if (!s || s.stopped)
    return;
  s.stopped = YES;
  if (s.starting)
    return;
  if (s.stream)
    [s.stream stopCaptureWithCompletionHandler:^(NSError *error) {
      dispatch_async(controlQueue(), ^{
        finish(s);
      });
    }];
  else
    finish(s);
}
static void windowDestroyed(AXObserverRef observer, AXUIElementRef element,
                            CFStringRef notification, void *context) {
  uint64_t token = (uint64_t)(uintptr_t)context;
  dispatch_async(controlQueue(), ^{
    OTWindowStream *s = sessions()[@(token)];
    if (s && !s.stopped) {
      goStreamUnavailable(token);
      stopSession(s);
    }
  });
}
// Resolve once and observe the exact AX window; never infer closure from
// absence in an app-wide enumeration (which can omit windows on other Spaces).
static void observeWindow(OTWindowStream *s, uint32_t window) {
  pid_t owner = ot_window_pid(window);
  if (owner <= 0 || !AXIsProcessTrusted())
    return;
  AXUIElementRef app = AXUIElementCreateApplication(owner);
  AXUIElementSetMessagingTimeout(app, 0.15);
  CFTypeRef values = NULL;
  if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &values) ==
          kAXErrorSuccess &&
      values && CFGetTypeID(values) == CFArrayGetTypeID()) {
    CFArrayRef windows = (CFArrayRef)values;
    for (CFIndex i = 0; i < CFArrayGetCount(windows); i++) {
      AXUIElementRef target =
          (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
      CGWindowID identifier = 0;
      if (_AXUIElementGetWindow(target, &identifier) != kAXErrorSuccess ||
          identifier != window)
        continue;
      AXObserverRef observer = NULL;
      if (AXObserverCreate(owner, windowDestroyed, &observer) ==
          kAXErrorSuccess) {
        if (AXObserverAddNotification(
                observer, target, kAXUIElementDestroyedNotification,
                (void *)(uintptr_t)s.token) == kAXErrorSuccess) {
          s.observer = observer;
          s.observedWindow = (AXUIElementRef)CFRetain(target);
          CFRunLoopAddSource(observationLoop(),
                             AXObserverGetRunLoopSource(observer),
                             kCFRunLoopCommonModes);
        } else
          CFRelease(observer);
      }
      break;
    }
  }
  if (values)
    CFRelease(values);
  CFRelease(app);
}
@implementation OTWindowStream
- (void)stream:(SCStream *)stream didStopWithError:(NSError *)error {
  dispatch_async(controlQueue(), ^{
    self.stopped = YES;
    finish(self);
  });
}
- (void)stream:(SCStream *)stream
    didOutputSampleBuffer:(CMSampleBufferRef)sample
                   ofType:(SCStreamOutputType)type {
  @autoreleasepool {
    if (type != SCStreamOutputTypeScreen || !CMSampleBufferIsValid(sample))
      return;
    CFTimeInterval now = NSProcessInfo.processInfo.systemUptime;
    if (now - self.lastFrame < 0.15)
      return;
    self.lastFrame = now;
    CVImageBufferRef buffer = CMSampleBufferGetImageBuffer(sample);
    if (!buffer)
      return;
    CFArrayRef attachments =
        CMSampleBufferGetSampleAttachmentsArray(sample, false);
    if (attachments && CFArrayGetCount(attachments) > 0) {
      NSDictionary *a =
          (__bridge NSDictionary *)CFArrayGetValueAtIndex(attachments, 0);
      NSNumber *status = a[SCStreamFrameInfoStatus];
      if (status && status.integerValue != SCFrameStatusComplete)
        return;
    }
    CIImage *image = [CIImage imageWithCVPixelBuffer:buffer];
    CGColorSpaceRef color = CGColorSpaceCreateDeviceRGB();
    NSData *png = [self.images PNGRepresentationOfImage:image
                                                 format:kCIFormatRGBA8
                                             colorSpace:color
                                                options:@{}];
    CGColorSpaceRelease(color);
    if (png) {
      NSString *url = [@"data:image/png;base64,"
          stringByAppendingString:[png base64EncodedStringWithOptions:0]];
      goStreamFrame(self.token, (char *)url.UTF8String);
    }
  }
}
@end
void ot_stream_start(uint64_t token, uint32_t window, int maxpx) {
  dispatch_async(controlQueue(), ^{
    if (@available(macOS 12.3, *)) {
      if (!CGPreflightScreenCaptureAccess()) {
        goStreamDone(token);
        return;
      }
      OTWindowStream *s = [OTWindowStream new];
      s.token = token;
      s.images = [CIContext contextWithOptions:nil];
      sessions()[@(token)] = s;
      observeWindow(s, window);
      [SCShareableContent
          getShareableContentExcludingDesktopWindows:YES
                                 onScreenWindowsOnly:NO
                                   completionHandler:^(
                                       SCShareableContent *content,
                                       NSError *error) {
                                     dispatch_async(controlQueue(), ^{
                                       if (s.stopped)
                                         return;
                                       SCWindow *target = nil;
                                       for (SCWindow *w in content.windows) {
                                         if (w.windowID == window) {
                                           target = w;
                                           break;
                                         }
                                       }
                                       if (error || !target) {
                                         stopSession(s);
                                         return;
                                       }
                                       SCContentFilter *filter =
                                           [[SCContentFilter alloc]
                                               initWithDesktopIndependentWindow:
                                                   target];
                                       SCStreamConfiguration *config =
                                           [SCStreamConfiguration new];
                                       CGFloat scale = MIN(
                                           1.0,
                                           (CGFloat)MAX(1, maxpx) /
                                               MAX(1,
                                                   MAX(target.frame.size.width,
                                                       target.frame.size
                                                           .height)));
                                       config.width = MAX(
                                           1, target.frame.size.width * scale);
                                       config.height = MAX(
                                           1, target.frame.size.height * scale);
                                       config.minimumFrameInterval =
                                           CMTimeMake(1, 6);
                                       config.queueDepth = 3;
                                       config.showsCursor = NO;
                                       s.stream = [[SCStream alloc]
                                           initWithFilter:filter
                                            configuration:config
                                                 delegate:s];
                                       NSError *outputError = nil;
                                       dispatch_queue_t frames =
                                           dispatch_queue_create(
                                               "com.optiontab.stream.frames",
                                               DISPATCH_QUEUE_SERIAL);
                                       if (![s.stream
                                                  addStreamOutput:s
                                                             type:
                                                                 SCStreamOutputTypeScreen
                                               sampleHandlerQueue:frames
                                                            error:
                                                                &outputError]) {
                                         stopSession(s);
                                         return;
                                       }
                                       s.starting = YES;
                                       [s.stream
                                           startCaptureWithCompletionHandler:^(
                                               NSError *startError) {
                                             dispatch_async(controlQueue(), ^{
                                               s.starting = NO;
                                               if (startError) {
                                                 s.stopped = YES;
                                                 finish(s);
                                               } else if (s.stopped) {
                                                 s.stopped = NO;
                                                 stopSession(s);
                                               }
                                             });
                                           }];
                                     });
                                   }];
    } else {
      goStreamDone(token);
    }
  });
}
void ot_stream_stop(uint64_t token) {
  dispatch_async(controlQueue(), ^{
    OTWindowStream *s = sessions()[@(token)];
    if (s)
      stopSession(s);
    else
      goStreamDone(token);
  });
}
