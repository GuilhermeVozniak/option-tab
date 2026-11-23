// Checks what ot_overlay_accept_first_click changes on real WKWebViews.
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "../../darwin_overlay_mouse.h"

static NSWindow *windowWithWebView(WKWebView **web) {
  NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 320, 200)
                                                 styleMask:NSWindowStyleMaskBorderless
                                                   backing:NSBackingStoreBuffered
                                                     defer:YES];
  NSView *container = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 320, 200)];
  *web = [[WKWebView alloc] initWithFrame:container.bounds];
  [container addSubview:*web];
  window.contentView = container;
  return window;
}

int main(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    WKWebView *overlayWeb = nil;
    WKWebView *otherWeb = nil;
    NSWindow *overlay = windowWithWebView(&overlayWeb);
    NSWindow *other = windowWithWebView(&otherWeb);
    NSWindow *plain = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 10, 10)
                                                  styleMask:NSWindowStyleMaskBorderless
                                                    backing:NSBackingStoreBuffered
                                                      defer:YES];
    NSView *plainView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 10, 10)];
    plain.contentView = plainView;

    NSEvent *click = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDown
                                        location:NSMakePoint(10, 10)
                                   modifierFlags:0
                                       timestamp:0
                                    windowNumber:overlay.windowNumber
                                         context:nil
                                     eventNumber:0
                                      clickCount:1
                                        pressure:1];
    if ([overlayWeb acceptsFirstMouse:click]) {
      fprintf(stderr, "a web view accepts the first click on its own; the fix is not needed\n");
      return 1;
    }

    // Nothing to prepare: no window, and a window without a web view.
    ot_overlay_accept_first_click(NULL);
    ot_overlay_accept_first_click((__bridge void *)plain);

    ot_overlay_accept_first_click((__bridge void *)overlay);
    ot_overlay_accept_first_click((__bridge void *)overlay);

    if (![overlayWeb acceptsFirstMouse:click]) {
      fprintf(stderr, "the overlay's web view still declines the first click\n");
      return 1;
    }
    if ([otherWeb acceptsFirstMouse:click]) {
      fprintf(stderr, "a web view in another window now accepts the first click\n");
      return 1;
    }
    if ([plainView acceptsFirstMouse:click]) {
      fprintf(stderr, "an ordinary view now accepts the first click\n");
      return 1;
    }
    printf("ok\n");
  }
  return 0;
}
