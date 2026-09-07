#import <AppKit/AppKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
@interface FixtureLyricsPanel:NSObject
@property BOOL canChooseFiles,canChooseDirectories,allowsMultipleSelection,canCreateDirectories;
@property NSArray *allowedContentTypes;
@property NSString *prompt;
@property NSURL *URL;
@property(copy) void (^completion)(NSModalResponse);
+(instancetype)openPanel;
-(void)beginWithCompletionHandler:(void (^)(NSModalResponse))completion;
-(void)cancel:(id)sender;
@end
static FixtureLyricsPanel *activeFixturePanel;
@implementation FixtureLyricsPanel
+(instancetype)openPanel{activeFixturePanel=[self new];return activeFixturePanel;}
-(void)beginWithCompletionHandler:(void (^)(NSModalResponse))completion{self.completion=completion;}
-(void)cancel:(id)sender{(void)sender;void (^completion)(NSModalResponse)=self.completion;self.completion=nil;if(completion)completion(NSModalResponseCancel);}
@end
#define NSOpenPanel FixtureLyricsPanel
#include "../../darwin_media_lyrics.m"
#undef NSOpenPanel
static void pump(void){CFRunLoopRunInMode(kCFRunLoopDefaultMode,0.005,true);}


@interface CountedURL : NSObject
@property NSURL *url;
@property int starts;
@property int stops;
@end
@implementation CountedURL
-(BOOL)startAccessingSecurityScopedResource{self.starts++;return YES;}
-(void)stopAccessingSecurityScopedResource{self.stops++;}
-(id)forwardingTargetForSelector:(SEL)selector{(void)selector;return self.url;}
@end
static NSDictionary *readCounted(NSURL *url,uint64_t device,uint64_t inode,BOOL importing){
 CountedURL *counted=[CountedURL new];counted.url=url;
 NSDictionary *result=lyricsReadURL((NSURL *)counted,device,inode,importing);
 NSCAssert(counted.starts==1&&counted.stops==1,@"unbalanced security scope");
 return result;
}
int main(int argc,char **argv){@autoreleasepool{
 NSCAssert(argc==2,@"fixture directory required");NSString *directory=@(argv[1]);
 NSURL *url=[NSURL fileURLWithPath:[directory stringByAppendingPathComponent:@"original.lrc"]];
 NSData *original=[@"[00:01.00]Original fixture line\n" dataUsingEncoding:NSUTF8StringEncoding];NSCAssert([original writeToURL:url atomically:NO],@"write fixture");
 NSDictionary *first=readCounted(url,0,0,YES);NSCAssert(!first[@"error"]&&[first[@"bookmark"] length],@"regular import %@",first);
 NSData *bookmark=[[NSData alloc]initWithBase64EncodedString:first[@"bookmark"] options:0];char *raw=ot_lyrics_read(bookmark.bytes,(int)bookmark.length,[first[@"device"] unsignedLongLongValue],[first[@"inode"] unsignedLongLongValue]);NSDictionary *loaded=[NSJSONSerialization JSONObjectWithData:[NSData dataWithBytes:raw length:strlen(raw)] options:0 error:nil];free(raw);NSCAssert(!loaded[@"error"]&&[loaded[@"data"]isEqual:first[@"data"]],@"bookmark reload %@",loaded);
 NSURL *link=[NSURL fileURLWithPath:[directory stringByAppendingPathComponent:@"link.lrc"]];NSCAssert(symlink(url.path.fileSystemRepresentation,link.path.fileSystemRepresentation)==0,@"create symlink");NSCAssert(readCounted(link,0,0,YES)[@"error"],@"symlink accepted");
 NSString *old=[directory stringByAppendingPathComponent:@"old.lrc"];NSCAssert(rename(url.path.fileSystemRepresentation,old.fileSystemRepresentation)==0,@"rename original");NSCAssert([original writeToURL:url atomically:NO],@"replacement");NSCAssert(readCounted(url,[first[@"device"] unsignedLongLongValue],[first[@"inode"] unsignedLongLongValue],NO)[@"error"],@"replacement accepted");
 NSCAssert([[NSMutableData dataWithLength:1048577]writeToURL:url atomically:NO],@"large fixture");NSCAssert(readCounted(url,0,0,YES)[@"error"],@"oversize accepted");
 NSCAssert([[NSData data]writeToURL:url atomically:NO],@"empty fixture");NSCAssert(readCounted(url,0,0,YES)[@"error"],@"empty accepted");
 void *chooser=ot_lyrics_choose_start();ot_lyrics_choose_cancel(chooser);
 char *cancelled=NULL;for(int i=0;i<100&&!cancelled;i++){pump();cancelled=ot_lyrics_choose_poll(chooser);}
 NSCAssert(cancelled,@"pre-start cancellation never completed");free(cancelled);ot_lyrics_choose_release(chooser);
 chooser=ot_lyrics_choose_start();for(int i=0;i<100&&!activeFixturePanel.completion;i++)pump();
 NSCAssert(activeFixturePanel.completion,@"substituted chooser not started");ot_lyrics_choose_cancel(chooser);
 cancelled=NULL;for(int i=0;i<100&&!cancelled;i++){pump();cancelled=ot_lyrics_choose_poll(chooser);}
 NSCAssert(cancelled&&!activeFixturePanel.completion,@"active cancellation did not join completion");free(cancelled);ot_lyrics_choose_release(chooser);activeFixturePanel=nil;
 printf("PASS native lyric files: regular read/bookmark reload, symlink/replacement/oversize/empty refusal, balanced scopes; substituted chooser cancellation before/after start, no visible chooser\n");
 return 0;
}}
