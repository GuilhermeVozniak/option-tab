#import "../../darwin_launcher_keyboard.m"
#include <assert.h>
@interface FixturePanel:NSObject
@property BOOL key;
@property int resigns;
@end
@implementation FixturePanel
-(BOOL)isKeyWindow{return self.key;}
-(void)makeKeyWindow{if(OTLauncherKeyboardCanKey(1))self.key=YES;}
-(BOOL)makeFirstResponder:(id)view{return view!=nil;}
-(void)resignKeyWindow{self.resigns++;self.key=NO;OTLauncherKeyboardResigned(1);}
@end
static FixturePanel *fixture;
static NSView *content;
static BOOL guardCurrent=YES,ordinary=YES;
NSWindow *OTLauncherKeyboardPanel(uint64_t token,const char*display){return token==1&&ordinary&&strcmp(display,"fixture")==0?(NSWindow*)fixture:nil;}
NSView *OTLauncherKeyboardContent(uint64_t token){return token==1?content:nil;}
int ot_go_launcher_keyboard_current(uintptr_t h){assert(h==1);return guardCurrent;}
@interface EscapeEvent:NSEvent
@end
@implementation EscapeEvent
-(NSEventType)type{return NSEventTypeKeyDown;}
-(unsigned short)keyCode{return 53;}
@end
int main(){@autoreleasepool{
 fixture=[FixturePanel new];content=[[NSView alloc]initWithFrame:NSMakeRect(0,0,100,100)];
 OTLauncherKeyboardPolicy p={.epoch=1,.session=1,.revision=1,.admission=1,.enabled=1};strcpy(p.display,"fixture");
 assert(!OTLauncherKeyboardCanKey(1));assert(!OTLauncherKeyboardCanKey(2));
 guardCurrent=NO;assert(!ot_launcher_keyboard_set(1,p,1));assert(!fixture.key);
 guardCurrent=YES;assert(ot_launcher_keyboard_set(1,p,1));assert(fixture.key);assert(ot_launcher_keyboard_valid(1,p));
 assert(OTLauncherKeyboardEvent(1,[EscapeEvent new]));assert(!fixture.key);assert(fixture.resigns==1);
 assert(!ot_launcher_keyboard_set(1,p,1)); // Escape is terminal for this admission.
 p.admission++;assert(ot_launcher_keyboard_set(1,p,1));
 [fixture resignKeyWindow];assert(!OTLauncherKeyboardCanKey(1));assert(!ot_launcher_keyboard_set(1,p,1));
 p.admission++;assert(ot_launcher_keyboard_set(1,p,1));
 ordinary=NO;assert(!ot_launcher_keyboard_valid(1,p));assert(!fixture.key);
 ordinary=YES;p.admission++;assert(ot_launcher_keyboard_set(1,p,1));
 OTLauncherKeyboardRetire(1,NO);assert(!fixture.key);assert(!ot_launcher_keyboard_set(1,p,1));
 p.admission++;assert(ot_launcher_keyboard_set(1,p,1));
 OTLauncherKeyboardPolicy old=p;old.admission--;old.enabled=0;
 assert(!ot_launcher_keyboard_set(1,old,1));assert(fixture.key);
 p.enabled=0;assert(ot_launcher_keyboard_set(1,p,1));assert(!fixture.key);
 p.admission++;p.enabled=1;assert(ot_launcher_keyboard_set(1,p,1));
 OTLauncherKeyboardRetire(1,YES);assert(!fixture.key);assert(!OTLauncherKeyboardCanKey(1));
 puts("launcher keyboard isolated policy PASS");
}}
