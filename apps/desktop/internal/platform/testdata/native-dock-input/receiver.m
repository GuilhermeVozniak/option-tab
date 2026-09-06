#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

static NSString *prefix;
static NSUInteger downs,ups;
static CGPoint savedPointer;
static BOOL posted=NO,restored=NO;
static void report(void) {
 NSString *state=[NSString stringWithFormat:@"%d %lu %lu\n",getpid(),(unsigned long)downs,(unsigned long)ups];
 [state writeToFile:[prefix stringByAppendingString:@".state"] atomically:YES encoding:NSUTF8StringEncoding error:nil];
}
@interface InputReceiver : NSView
@end
@implementation InputReceiver
- (BOOL)acceptsFirstMouse:(NSEvent *)event {return YES;}
- (void)mouseDown:(NSEvent *)event {downs++;report();}
- (void)mouseUp:(NSEvent *)event {ups++;report();}
- (void)drawRect:(NSRect)rect {[[NSColor colorWithRed:0.1 green:0.35 blue:0.7 alpha:1] setFill];NSRectFill(rect);}
@end
int main(int argc,const char **argv) {
 @autoreleasepool {
  if(argc!=2)return 2;
  prefix=[NSString stringWithUTF8String:argv[1]];
  [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  NSRect screen=NSScreen.mainScreen.frame;
  NSPanel *panel=[[NSPanel alloc] initWithContentRect:NSMakeRect(120,screen.size.height-280,220,120) styleMask:NSWindowStyleMaskBorderless|NSWindowStyleMaskNonactivatingPanel backing:NSBackingStoreBuffered defer:NO];
  panel.level=NSFloatingWindowLevel;panel.contentView=[[InputReceiver alloc] initWithFrame:NSMakeRect(0,0,220,120)];[panel orderFrontRegardless];
  CGEventRef point=CGEventCreate(NULL);savedPointer=CGEventGetLocation(point);CFRelease(point);report();
  [NSTimer scheduledTimerWithTimeInterval:0.05 repeats:YES block:^(NSTimer *timer){
   NSString *command=[NSString stringWithContentsOfFile:[prefix stringByAppendingString:@".command"] encoding:NSUTF8StringEncoding error:nil];
   if([command isEqualToString:@"click"]&&!posted) {
    posted=YES;
    for(int i=0;i<2;i++) {
     CGEventRef e=CGEventCreateMouseEvent(NULL,i?kCGEventLeftMouseUp:kCGEventLeftMouseDown,CGPointMake(220,220),kCGMouseButtonLeft);
     // This is deliberately a synthetic event and must pass the production tap.
     CGEventSetIntegerValueField(e,kCGEventSourceUnixProcessID,getpid());
     CGEventPost(kCGSessionEventTap,e);CFRelease(e);
    }
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,500*NSEC_PER_MSEC),dispatch_get_main_queue(),^{CGWarpMouseCursorPosition(savedPointer);restored=YES;});
   }
   if([command isEqualToString:@"stop"]) {
    if(!restored)CGWarpMouseCursorPosition(savedPointer);
    [panel orderOut:nil];[NSApp terminate:nil];
   }
  }];
  [NSApp run];
 }
 return 0;
}
