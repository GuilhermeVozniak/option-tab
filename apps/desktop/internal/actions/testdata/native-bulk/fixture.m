#import <Cocoa/Cocoa.h>
#import <libproc.h>
#import <unistd.h>

// A genuine edited document. No file is opened and all write paths refuse data.
@interface BulkDocument : NSDocument
@property NSUInteger closeChecks;
@property NSUInteger writeAttempts;
@end
@implementation BulkDocument
+ (BOOL)autosavesInPlace { return NO; }
+ (BOOL)autosavesDrafts { return NO; }
- (NSString *)autosavingFileType { return nil; }
- (void)canCloseDocumentWithDelegate:(id)delegate shouldCloseSelector:(SEL)selector contextInfo:(void *)context {
    self.closeChecks++;
    [super canCloseDocumentWithDelegate:delegate shouldCloseSelector:selector contextInfo:context];
}
- (BOOL)refuseWrite:(NSError **)error {
    self.writeAttempts++;
    if (error) *error = [NSError errorWithDomain:@"NativeBulkFixture" code:1 userInfo:@{NSLocalizedDescriptionKey:@"In-memory fixture refuses document writes"}];
    return NO;
}
- (NSData *)dataOfType:(NSString *)type error:(NSError **)error { [self refuseWrite:error]; return nil; }
- (BOOL)writeToURL:(NSURL *)url ofType:(NSString *)type error:(NSError **)error { return [self refuseWrite:error]; }
- (BOOL)writeToURL:(NSURL *)url ofType:(NSString *)type forSaveOperation:(NSSaveOperationType)op originalContentsURL:(NSURL *)original error:(NSError **)error { return [self refuseWrite:error]; }
- (BOOL)writeSafelyToURL:(NSURL *)url ofType:(NSString *)type forSaveOperation:(NSSaveOperationType)op error:(NSError **)error { return [self refuseWrite:error]; }
@end

static NSUInteger buttonCount(NSView *view) {
    NSUInteger count = [view isKindOfClass:NSButton.class] ? 1 : 0;
    for (NSView *child in view.subviews) count += buttonCount(child);
    return count;
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc != 6) return 64;
        // Kernel-delivered bound survives a blocked AppKit main run loop.
        alarm(30);
        NSString *path = @(argv[1]), *nonce = @(argv[2]), *role = @(argv[3]), *scenario = @(argv[4]);
        pid_t baseline = (pid_t)atoi(argv[5]), parent = getppid();
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:30];
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
        __block BOOL activated = NSApp.active, foregroundChanged = NO;
        NSNotificationCenter *center = NSNotificationCenter.defaultCenter;
        [center addObserverForName:NSApplicationDidBecomeActiveNotification object:NSApp queue:nil usingBlock:^(NSNotification *note) { activated = YES; }];
        [NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceDidActivateApplicationNotification object:nil queue:nil usingBlock:^(NSNotification *note) {
            NSRunningApplication *app = note.userInfo[NSWorkspaceApplicationKey];
            if (app.processIdentifier != baseline) foregroundChanged = YES;
        }];
        NSMutableArray<NSWindow *> *windows = [NSMutableArray array];
        NSMutableArray<NSNumber *> *ids = [NSMutableArray array];
        NSMutableSet<NSNumber *> *closed = [NSMutableSet set];
        [center addObserverForName:NSWindowWillCloseNotification object:nil queue:nil usingBlock:^(NSNotification *note) {
            NSUInteger i = [windows indexOfObjectIdenticalTo:note.object];
            if (i != NSNotFound) [closed addObject:ids[i]];
        }];
        BOOL target = [role isEqualToString:@"target"], minimize = [scenario isEqualToString:@"minimize"];
        NSUInteger count = target && minimize ? 3 : 2;
        BulkDocument *document = nil;
        for (NSUInteger i = 0; i < count; i++) {
            NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(60 + i * 40, 70 + (target ? 0 : 260), 300, 180)
                styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
                backing:NSBackingStoreBuffered defer:NO];
            window.releasedWhenClosed = NO;
            window.restorable = NO;
            window.title = [NSString stringWithFormat:@"Bulk fixture %@ %lu", role, (unsigned long)i];
            if (target && !minimize && i == 0) {
                document = [BulkDocument new];
                document.fileType = @"public.plain-text";
                NSWindowController *controller = [[NSWindowController alloc] initWithWindow:window];
                [document addWindowController:controller];
                [NSDocumentController.sharedDocumentController addDocument:document];
                [document updateChangeCount:NSChangeDone];
            }
            [windows addObject:window];
            [ids addObject:@(window.windowNumber)];
            [window orderFrontRegardless];
        }
        if (target && minimize) [windows[0] miniaturize:nil];
        struct proc_bsdinfo info = {0};
        if (proc_pidinfo(getpid(), PROC_PIDTBSDINFO, 0, &info, sizeof(info)) != sizeof(info)) return 65;
        NSDictionary *identity = @{ @"PID":@(getpid()), @"StartSeconds":@(info.pbi_start_tvsec), @"StartMicros":@(info.pbi_start_tvusec) };
        __block uint64_t sequence = 0;
        void (^publish)(void) = ^{
            // Independent lifetime cap also covers a parent go-test timeout/panic.
            if (getppid() != parent || [deadline timeIntervalSinceNow] <= 0) _exit(0);
            pid_t foreground = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
            if (foreground != baseline) foregroundChanged = YES;
            if (NSApp.active) activated = YES;
            NSMutableArray *states = [NSMutableArray array];
            for (NSUInteger i=0; i<windows.count; i++) {
                NSWindow *w = windows[i];
                [states addObject:@{@"id":ids[i], @"visible":@(w.visible), @"minimized":@(w.miniaturized), @"closed":@([closed containsObject:ids[i]])}];
            }
            NSMutableDictionary *state = [@{@"sequence":@(++sequence), @"pid":@(getpid()), @"nonce":nonce, @"role":role,
                @"bundle":NSBundle.mainBundle.bundleIdentifier ?: @"", @"executable":NSBundle.mainBundle.executablePath.stringByResolvingSymlinksInPath,
                @"process":identity, @"foreground":@(foreground), @"activated":@(activated), @"foregroundChanged":@(foregroundChanged), @"windows":states} mutableCopy];
            if (document) {
                NSWindow *w = windows[0], *sheet = w.attachedSheet;
                state[@"document"] = @{@"windowID":ids[0], @"edited":@(document.documentEdited), @"closeChecks":@(document.closeChecks),
                    @"writeAttempts":@(document.writeAttempts), @"sheetID":@(sheet ? sheet.windowNumber : 0), @"sheetVisible":@(sheet.visible),
                    @"isSheet":@(sheet.sheet), @"buttonCount":@(sheet ? buttonCount(sheet.contentView) : 0)};
            }
            NSData *json = [NSJSONSerialization dataWithJSONObject:state options:NSJSONWritingPrettyPrinted error:NULL];
            if (![json writeToFile:path atomically:YES]) _exit(66);
            if (activated || foregroundChanged) _exit(67);
        };
        publish();
        NSTimer *timer = [NSTimer timerWithTimeInterval:0.025 repeats:YES block:^(NSTimer *t) { publish(); }];
        [NSRunLoop.mainRunLoop addTimer:timer forMode:NSRunLoopCommonModes];
        [NSApp run];
    }
    return 0;
}
