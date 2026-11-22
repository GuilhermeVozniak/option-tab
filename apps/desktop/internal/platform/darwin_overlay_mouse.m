// The switcher overlay is shown without activating Option Tab, so the app the
// user came from keeps the keyboard. AppKit then treats the overlay as a
// background window, and a web view declines the first click on one: the
// click only activated Option Tab and had to be repeated to pick a window.

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "darwin_overlay_mouse.h"

static char kAcceptsFirstClickKey;
static BOOL (*gOriginalAcceptsFirstMouse)(id, SEL, NSEvent *);

static BOOL acceptsFirstMouse(id view, SEL selector, NSEvent *event) {
  if (objc_getAssociatedObject(view, &kAcceptsFirstClickKey) != nil) return YES;
  return gOriginalAcceptsFirstMouse != NULL ? gOriginalAcceptsFirstMouse(view, selector, event)
                                            : NO;
}

// Replaces the web view class's answer once; only marked views change theirs.
// The view's class cannot be subclassed per instance instead: WebKit observes
// its views with key-value observing, which already swaps the class.
static void installOnce(Class webViewClass) {
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    SEL selector = @selector(acceptsFirstMouse:);
    Method method = class_getInstanceMethod(webViewClass, selector);
    if (method == NULL) return;
    gOriginalAcceptsFirstMouse = (BOOL (*)(id, SEL, NSEvent *))method_getImplementation(method);
    const char *types = method_getTypeEncoding(method);
    // Adding fails when the class has its own implementation; replace that
    // one. Never touch an inherited method, which belongs to every NSView.
    if (!class_addMethod(webViewClass, selector, (IMP)acceptsFirstMouse, types)) {
      method_setImplementation(method, (IMP)acceptsFirstMouse);
    }
  });
}

static NSView *webViewIn(NSView *view, Class webViewClass) {
  if (view == nil) return nil;
  if ([view isKindOfClass:webViewClass]) return view;
  for (NSView *child in view.subviews) {
    NSView *found = webViewIn(child, webViewClass);
    if (found != nil) return found;
  }
  return nil;
}

void ot_overlay_accept_first_click(void *window) {
  Class webViewClass = NSClassFromString(@"WKWebView");
  if (window == NULL || webViewClass == Nil) return;
  NSWindow *w = (__bridge NSWindow *)window;
  NSView *web = webViewIn(w.contentView, webViewClass);
  if (web == nil) return;
  installOnce(webViewClass);
  objc_setAssociatedObject(web, &kAcceptsFirstClickKey, @YES, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
