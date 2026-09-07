#import <Cocoa/Cocoa.h>
static BOOL fixtureCurrent = YES;
static void (^beforeApply)(void);
static void (^fadeDone)(void);
#define OT_MATERIAL_CURRENT(token) fixtureCurrent
#define OT_MATERIAL_BEFORE_APPLY()                                             \
  do {                                                                         \
    if (beforeApply)                                                           \
      beforeApply();                                                           \
  } while (0)
#define OT_MATERIAL_FADE(effect, ...)                                          \
  do {                                                                         \
    effect.alphaValue = 0;                                                     \
    fadeDone = [(__VA_ARGS__) copy];                                           \
  } while (0)
#include "../../darwin_launcher_gestures.m"
#include "../../darwin_dock_panel.m"
#include "../../darwin_material.m"
#include "../../darwin_media_panel.m"
uint64_t ot_launcher_space_id(const char *display) { abort(); }
int ot_launcher_space_ordinary(const char *display) { abort(); }
@interface MaterialFixtureHost : NSObject
@property NSView *contentView;
@property NSAppearance *appearance;
@end
@implementation MaterialFixtureHost
@end
static void check(BOOL v, NSString *reason) {
  if (!v) {
    fprintf(stderr, "%s\n", reason.UTF8String);
    exit(1);
  }
}
int main(void) {
  @autoreleasepool {
    MaterialFixtureHost *host = [MaterialFixtureHost new];
    NSView *content =
        [[NSView alloc] initWithFrame:NSMakeRect(-100, 30, 400, 300)];
    NSView *web = [[NSView alloc] initWithFrame:NSMakeRect(3, 4, 390, 290)];
    [content addSubview:web];
    host.contentView = content;
    NSAppearance *original =
        [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
    host.appearance = original;
    NSRect originalFrame = content.frame, webFrame = web.frame;
    uint64_t token =
        materialAttach((NSWindow *)host, content, (NSWindow *)host, 0);
    check(token != 0, @"attach refused inert owned host");
    check(ot_material_apply(token, 1, 1, 1, "light", 64, 10, 20, 200, 100, 1) ==
              1,
          @"system apply refused");
    OTMaterialRecord *r = materialRecords()[@(token)];
    check(NSEqualRects(r.effect.frame, NSMakeRect(10, 180, 200, 100)),
          @"host-local top-left conversion used global frame");
    check(r.effect.layer.cornerRadius == 50 && r.effect.layer.masksToBounds,
          @"radius not clipped to shorter edge");
    check(content.subviews.firstObject == r.effect &&
              content.subviews.lastObject == web,
          @"effect is not behind content");
    check([r.effect hitTest:NSMakePoint(20, 20)] == nil,
          @"material intercepted input");
    check(ot_material_apply(token, 1, 2, 1, "dark", 18, 350, 0, 100, 20, 1) ==
              0,
          @"outside rect accepted");
    check(ot_material_apply(token, 1, 2, 0, "system", 0, 0, 0, 0, 0, 1) == 1 &&
              r.effect == nil,
          @"zero-rect solid did not remove effect");
    check(ot_material_apply(token, 1, 3, 1, "dark", 18, 0, 0, 100, 80, 1) == 1,
          @"second system apply refused");
    check(ot_material_retire(token, 1, 3, 180, 1) == 1, @"retire refused");
    check(ot_material_apply(token, 1, 4, 1, "dark", 18, 0, 0, 100, 80, 1) == 0,
          @"retired session reapplied");
    check(ot_material_apply(token, 2, 4, 1, "dark", 18, 0, 0, 120, 80, 1) == 1,
          @"successor apply refused");
    if (fadeDone) {
      fadeDone();
      fadeDone = nil;
    }
    check(r.effect != nil && r.effect.alphaValue == 1,
          @"old fade removed successor");
    beforeApply = ^{
      fixtureCurrent = NO;
    };
    check(ot_material_apply(token, 2, 5, 0, "system", 0, 0, 0, 0, 0, 1) == 0 &&
              r.effect != nil,
          @"cancelled preparation mutated host");
    beforeApply = nil;
    fixtureCurrent = YES;
    ot_material_close(token);
    ot_material_close(token);
    check(content.subviews.count == 1 && content.subviews[0] == web &&
              NSEqualRects(content.frame, originalFrame) &&
              NSEqualRects(web.frame, webFrame) && host.appearance == original,
          @"close did not restore original content/theme");
    check(ot_material_apply(token, 3, 9, 1, "dark", 0, 0, 0, 20, 20, 1) == 0,
          @"retired native token accepted");
    uint64_t next =
        materialAttach((NSWindow *)host, content, (NSWindow *)host, 0);
    check(next && next != token, @"recreated surface reused token");
    [NSNotificationCenter.defaultCenter
        postNotificationName:NSWindowWillCloseNotification
                      object:host];
    check(materialRecords()[@(next)] == nil,
          @"host close leaked material registry");
    uint64_t swapped =
        materialAttach((NSWindow *)host, content, (NSWindow *)host, 0);
    NSView *successor = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 50, 50)];
    NSAppearance *successorTheme =
        [NSAppearance appearanceNamed:NSAppearanceNameAqua];
    host.contentView = successor;
    host.appearance = successorTheme;
    ot_material_close(swapped);
    check(host.contentView == successor && host.appearance == successorTheme,
          @"old surface close modified successor content theme");
    host.contentView = content;
    OTDockPanelRecord *panel = [OTDockPanelRecord new];
    panel.panel = (OTDockPanel *)host;
    panel.host = (NSWindow *)host;
    panel.content = content;
    panelRecords()[@80] = panel;
    void *w = NULL, *c = NULL, *h = NULL;
    check(ot_preview_material_target(80, &w, &c, &h),
          @"ordinary preview refused");
    panel.launcherDisplay = @"display";
    check(!ot_preview_material_target(80, &w, &c, &h),
          @"launcher accepted as preview");
    panel.launcherDisplay = nil;
    panel.materialExcluded = YES;
    check(!ot_preview_material_target(80, &w, &c, &h),
          @"media accepted as preview");
    [panelRecords() removeObjectForKey:@80];
    puts("PASS: material rect/layer/theme/solid, session retirement/fade, "
         "cancellation, exact cleanup and policy refusal; inert views only");
  }
}

int ot_go_launcher_gesture_policy_current(uintptr_t handle) { return 0; }
#import "../../darwin_launcher_keyboard.m"
int ot_go_launcher_keyboard_current(uintptr_t handle) { return 0; }
