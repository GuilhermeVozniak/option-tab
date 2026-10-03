//go:build darwin

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import "darwin_session.h"

// Notification callbacks retain no native context. The observer is independently
// retained by Go; close removes every token and closes its synchronized mailbox.
@interface OTSessionObserver : NSObject
@property(nonatomic) NSLock *lock;
@property(nonatomic) NSMutableArray *tokens;
@property(nonatomic) BOOL closed;
@property(nonatomic) uint64_t revision;
@property(nonatomic) int sessionEvent;
@property(nonatomic) int screenEvent;
@property(nonatomic) int sleepEvent;
@property(nonatomic) int lockEvent;
- (void)note:(int)kind state:(int)state;
- (void)close;
@end
@implementation OTSessionObserver
- (instancetype)init {
 self=[super init];if(!self)return nil;
 self.lock=[NSLock new];self.tokens=[NSMutableArray array];
 __weak OTSessionObserver *weakSelf=self;
 NSNotificationCenter *workspace=NSWorkspace.sharedWorkspace.notificationCenter;
 NSArray *names=@[NSWorkspaceSessionDidResignActiveNotification,NSWorkspaceSessionDidBecomeActiveNotification,NSWorkspaceScreensDidSleepNotification,NSWorkspaceScreensDidWakeNotification,NSWorkspaceWillSleepNotification,NSWorkspaceDidWakeNotification];
 for(NSUInteger i=0;i<names.count;i++) {
  int kind=(int)(i/2);int state=i%2==0?1:2;
  id token=[workspace addObserverForName:names[i] object:nil queue:nil usingBlock:^(NSNotification *note){[weakSelf note:kind state:state];}];
  if(token)[self.tokens addObject:@[workspace,token]];
 }
 // These lock names and the dictionary key below are undocumented macOS hints.
 // They supplement public session/sleep signals; they are not a security API.
 NSDistributedNotificationCenter *distributed=NSDistributedNotificationCenter.defaultCenter;
 for(NSString *name in @[@"com.apple.screenIsLocked",@"com.apple.screenIsUnlocked"]) {
  int state=[name isEqualToString:@"com.apple.screenIsLocked"]?1:2;
  id token=[distributed addObserverForName:name object:nil queue:nil usingBlock:^(NSNotification *note){[weakSelf note:3 state:state];}];
  if(token)[self.tokens addObject:@[distributed,token]];
 }
 return self;
}
- (void)note:(int)kind state:(int)state {
 [self.lock lock];
 if(!self.closed){self.revision++;switch(kind){case 0:self.sessionEvent=state;break;case 1:self.screenEvent=state;break;case 2:self.sleepEvent=state;break;case 3:self.lockEvent=state;break;}}
 [self.lock unlock];
}
- (void)close {
 [self.lock lock];self.closed=YES;[self.lock unlock];
 for(NSArray *entry in self.tokens)[entry[0] removeObserver:entry[1]];
 [self.tokens removeAllObjects];
}
@end

void *ot_session_observer_create(void) {
 @autoreleasepool { return (__bridge_retained void *)[OTSessionObserver new]; }
}

OTSessionSnapshot ot_session_observer_poll(void *pointer) {
 @autoreleasepool {
  OTSessionSnapshot out={0};out.lock_state=-1;
  OTSessionObserver *observer=(__bridge OTSessionObserver *)pointer;
  if(!observer)return out;
  // Take the state snapshot while callbacks cannot interleave. CGSession is a
  // local WindowServer query; there are no AX traversals or application calls.
  [observer.lock lock];
  if(observer.closed){[observer.lock unlock];return out;}
  CFDictionaryRef session=CGSessionCopyCurrentDictionary();
  if(session){
   CFTypeRef onConsole=CFDictionaryGetValue(session,kCGSessionOnConsoleKey);
   CFTypeRef login=CFDictionaryGetValue(session,kCGSessionLoginDoneKey);
   out.valid=onConsole && login && CFGetTypeID(onConsole)==CFBooleanGetTypeID() && CFGetTypeID(login)==CFBooleanGetTypeID();
   if(out.valid){out.on_console=CFBooleanGetValue(onConsole);out.login_done=CFBooleanGetValue(login);}
   CFTypeRef locked=CFDictionaryGetValue(session,CFSTR("CGSSessionScreenIsLocked"));
   if(locked && CFGetTypeID(locked)==CFBooleanGetTypeID())out.lock_state=CFBooleanGetValue(locked)?1:0;
   CFRelease(session);
  }
  out.revision=observer.revision;out.session_event=observer.sessionEvent;out.screen_event=observer.screenEvent;out.sleep_event=observer.sleepEvent;out.lock_event=observer.lockEvent;
  observer.sessionEvent=0;observer.screenEvent=0;observer.sleepEvent=0;observer.lockEvent=0;
  [observer.lock unlock];
  return out;
 }
}
void ot_session_observer_destroy(void *pointer) {
 @autoreleasepool { if(pointer){OTSessionObserver *observer=(__bridge_transfer OTSessionObserver *)pointer;[observer close];} }
}
