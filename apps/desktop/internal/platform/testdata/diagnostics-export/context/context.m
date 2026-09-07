#import "context.h"
#import <Cocoa/Cocoa.h>
static NSMutableArray *pending;
static int panels;
static BOOL cancelAtCommit;
extern void goDiagnosticFixtureCancel(void);
static void beforeCommit(void) {
  if (cancelAtCommit)
    goDiagnosticFixtureCancel();
}
#define OT_DIAGNOSTIC_BEFORE_COMMIT(g) beforeCommit()
static void schedule(void (^work)(void)) {
  if (!pending)
    pending = [NSMutableArray new];
  [pending addObject:[work copy]];
}
static NSSavePanel *forbiddenPanel(void) {
  panels++;
  return nil;
}
#define OT_DIAGNOSTIC_SCHEDULE_MAIN(work) schedule(work)
#define OT_DIAGNOSTIC_MAKE_PANEL() forbiddenPanel()
#define ot_diagnostics_main_thread fixture_main_thread
#define ot_diagnostics_save_start fixture_save_start
#define ot_diagnostics_save_cancel fixture_save_cancel
#define ot_diagnostics_save_poll fixture_save_poll
#define ot_diagnostics_save_release fixture_save_release
#define OTDiagnosticSave FixtureDiagnosticSave
#include "../../../darwin_diagnostics_export.m"
void *context_start(uintptr_t token) {
  return fixture_save_start("{}", 2, token);
}
void context_pump(void) {
  while (pending.count) {
    void (^work)(void) = pending.firstObject;
    [pending removeObjectAtIndex:0];
    work();
  }
}
int context_result(void *value) { return fixture_save_poll(value); }
int context_panels(void) { return panels; }
void context_release(void *value) { fixture_save_release(value); }

int context_write(uintptr_t token, const char *path) {
  FixtureDiagnosticSave *owner = [FixtureDiagnosticSave new];
  owner.lock = [NSLock new];
  owner.admission = token;
  owner.data = [@"reviewed" dataUsingEncoding:NSUTF8StringEncoding];
  cancelAtCommit = YES;
  diagnosticWrite(owner, [NSURL fileURLWithPath:@(path)]);
  cancelAtCommit = NO;
  return owner.result;
}
