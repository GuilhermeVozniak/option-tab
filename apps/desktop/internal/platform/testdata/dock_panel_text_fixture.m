#import <Cocoa/Cocoa.h>
#include <stdio.h>
#import <ApplicationServices/ApplicationServices.h>
@interface SmokeText : NSTextView
@end
@implementation SmokeText
- (void)mouseDown:(NSEvent *)event { puts("fixture-mouse");fflush(stdout);[super mouseDown:event]; }
- (void)keyDown:(NSEvent *)event { if([event.characters isEqualToString:@"x"]){puts("fixture-key-x");fflush(stdout);}[super keyDown:event]; }
@end
int main(int argc,char **argv){
 if(argc==2&&strcmp(argv[1],"--pointer")==0){CGEventRef event=CGEventCreate(NULL);CGPoint p=CGEventGetLocation(event);printf("%.9f %.9f\n",p.x,p.y);CFRelease(event);return 0;}
 if(argc==4&&strcmp(argv[1],"--warp")==0){return CGWarpMouseCursorPosition(CGPointMake(atof(argv[2]),atof(argv[3])));}
 @autoreleasepool{NSApplication *app=[NSApplication sharedApplication];[app setActivationPolicy:NSApplicationActivationPolicyRegular];int index=atoi(getenv("DOCK_PANEL_SCREEN_INDEX")?:"0");NSScreen *screen=NSScreen.screens[index];CGFloat top=NSMaxY(screen.frame);CGFloat left=NSMinX(screen.frame);NSWindow *window=[[NSWindow alloc]initWithContentRect:NSMakeRect(left+80,top-100-600,800,600) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];window.releasedWhenClosed=NO;window.title=@"Option Tab disposable text fixture";SmokeText *text=[[SmokeText alloc]initWithFrame:NSMakeRect(0,0,800,600)];text.string=@"Disposable text focus fixture\n";window.contentView=text;[window makeFirstResponder:text];[window makeKeyAndOrderFront:nil];[app activateIgnoringOtherApps:YES];printf("fixture-ready %d %ld\n",getpid(),(long)window.windowNumber);fflush(stdout);[app run];}return 0;}
