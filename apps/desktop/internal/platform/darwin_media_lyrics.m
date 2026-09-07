//go:build darwin

#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#import <sys/stat.h>
#import <fcntl.h>
#import <unistd.h>
#import "darwin_media_lyrics.h"

int ot_lyrics_main_thread(void) { return NSThread.isMainThread ? 1 : 0; }

static char *lyricsJSON(NSDictionary *value) {
    NSData *data = [NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
    return strdup(data ? [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String : "{}");
}
static NSDictionary *lyricsError(NSString *message) {
    return @{@"error":message ?: @"Lyric file unavailable"};
}
// Balanced for both sandbox-issued scope and ordinary explicitly selected files.
static NSDictionary *lyricsReadURL(NSURL *url, uint64_t device, uint64_t inode, BOOL importing) {
    BOOL accessed = [url startAccessingSecurityScopedResource];
    int fd = -1;
    @try {
        NSString *path = url.URLByStandardizingPath.path;
        if (!url.isFileURL || ![path isEqual:url.URLByResolvingSymlinksInPath.path] || ![path.pathExtension.lowercaseString isEqual:@"lrc"])
            return lyricsError(@"Select a regular .lrc file, not a symbolic link");
        fd = open(path.fileSystemRepresentation, O_RDONLY | O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK);
        struct stat before, after, named;
        if (fd < 0 || fstat(fd, &before) || !S_ISREG(before.st_mode))
            return lyricsError(@"Lyric file is missing or access was revoked");
        if (before.st_size <= 0 || before.st_size > 1048576)
            return lyricsError(@"Lyric file is empty or exceeds 1 MiB");
        if (!importing && ((uint64_t)before.st_dev != device || before.st_ino != inode))
            return lyricsError(@"Lyric file identity changed");
        NSMutableData *data = [NSMutableData dataWithLength:(NSUInteger)before.st_size];
        size_t offset = 0;
        while (offset < data.length) {
            ssize_t count = read(fd, (char *)data.mutableBytes + offset, data.length-offset);
            if (count < 0 && errno == EINTR) continue;
            if (count <= 0) return lyricsError(@"Lyric file changed while reading");
            offset += (size_t)count;
        }
        if (fstat(fd,&after) || lstat(path.fileSystemRepresentation,&named) ||
            before.st_dev != named.st_dev || before.st_ino != named.st_ino || !S_ISREG(named.st_mode) ||
            before.st_size != after.st_size || before.st_mtimespec.tv_sec != after.st_mtimespec.tv_sec ||
            before.st_mtimespec.tv_nsec != after.st_mtimespec.tv_nsec)
            return lyricsError(@"Lyric file changed while reading");
        NSError *error = nil;
        NSData *bookmark = importing ? [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope | NSURLBookmarkCreationSecurityScopeAllowOnlyReadAccess includingResourceValuesForKeys:nil relativeToURL:nil error:&error] : nil;
        if (importing && !bookmark) return lyricsError(error.localizedDescription);
        return @{@"data":[data base64EncodedStringWithOptions:0], @"bookmark":[bookmark base64EncodedStringWithOptions:0] ?: @"", @"device":@((uint64_t)before.st_dev), @"inode":@(before.st_ino)};
    } @finally {
        if (fd >= 0) close(fd);
        if (accessed) [url stopAccessingSecurityScopedResource];
    }
}
char *ot_lyrics_read(const void *bytes, int length, uint64_t device, uint64_t inode) {
    @autoreleasepool {
        if (!bytes || length <= 0 || length > 1048576) return lyricsJSON(lyricsError(@"Lyric bookmark invalid"));
        NSData *data = [NSData dataWithBytes:bytes length:(NSUInteger)length];
        BOOL stale = NO;
        NSError *error = nil;
        NSURL *url = [NSURL URLByResolvingBookmarkData:data options:NSURLBookmarkResolutionWithSecurityScope | NSURLBookmarkResolutionWithoutUI | NSURLBookmarkResolutionWithoutMounting relativeToURL:nil bookmarkDataIsStale:&stale error:&error];
        if (!url || stale) return lyricsJSON(lyricsError(@"Lyric bookmark is stale or revoked; choose the file again"));
        return lyricsJSON(lyricsReadURL(url,device,inode,NO));
    }
}

@interface OTMediaLyricsChooser : NSObject
@property NSOpenPanel *panel;
@property BOOL cancelled;
@property NSString *result;
@end
@implementation OTMediaLyricsChooser
@end
static void lyricsFinish(OTMediaLyricsChooser *owner, NSDictionary *value) {
    char *encoded = lyricsJSON(value);
    @synchronized(owner) {
        if (!owner.result) owner.result = @(encoded);
    }
    free(encoded);
}
void *ot_lyrics_choose_start(void) {
    OTMediaLyricsChooser *owner = [OTMediaLyricsChooser new];
    void *handle = (__bridge_retained void *)owner;
    dispatch_async(dispatch_get_main_queue(), ^{
        @synchronized(owner) {
            if (owner.cancelled) { lyricsFinish(owner,lyricsError(@"Lyric import cancelled")); return; }
        }
        NSOpenPanel *panel = NSOpenPanel.openPanel;
        owner.panel = panel;
        panel.canChooseFiles = YES;
        panel.canChooseDirectories = NO;
        panel.allowsMultipleSelection = NO;
        panel.canCreateDirectories = NO;
        panel.allowedContentTypes = @[[UTType typeWithFilenameExtension:@"lrc"] ?: UTTypePlainText];
        [panel beginWithCompletionHandler:^(NSModalResponse response) {
            NSURL *selected = panel.URL;
            owner.panel = nil;
            @synchronized(owner) {
                if (owner.cancelled || response != NSModalResponseOK || !selected) {
                    lyricsFinish(owner,lyricsError(@"Lyric import cancelled")); return;
                }
            }
            // File I/O and bookmark creation do not block AppKit's main thread.
            dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0), ^{
                NSDictionary *result = lyricsReadURL(selected,0,0,YES);
                @synchronized(owner) {
                    lyricsFinish(owner,owner.cancelled ? lyricsError(@"Lyric import cancelled") : result);
                }
            });
        }];
    });
    return handle;
}
void ot_lyrics_choose_cancel(void *value) {
    OTMediaLyricsChooser *owner = (__bridge OTMediaLyricsChooser *)value;
    @synchronized(owner) { owner.cancelled = YES; }
    dispatch_async(dispatch_get_main_queue(), ^{ [owner.panel cancel:nil]; });
}
char *ot_lyrics_choose_poll(void *value) {
    OTMediaLyricsChooser *owner = (__bridge OTMediaLyricsChooser *)value;
    @synchronized(owner) { return owner.result ? strdup(owner.result.UTF8String) : NULL; }
}
void ot_lyrics_choose_release(void *value) {
    if (value) CFBridgingRelease(value);
}
