//go:build darwin

#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>
#import "darwin_media.h"
extern int ot_media_guard(uintptr_t token);

static NSString *bundle(NSString *p) {
 if ([p isEqual:@"music"]) return @"com.apple.Music";
 if ([p isEqual:@"spotify"]) return @"com.spotify.client";
 return nil;
}
static char *json(NSDictionary *value) {
 NSData *d=[NSJSONSerialization dataWithJSONObject:value options:0 error:nil];
 return strdup(d ? [[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding].UTF8String : "{}");
}
static NSDictionary *failure(NSString *status, NSString *reason) {return @{@"status":status,@"reason":reason};}
static NSRunningApplication *running(NSString *p) {
 NSString *b=bundle(p); if(!b)return nil;
#ifdef OT_MEDIA_FIXTURE
 return ot_fixture_running(p);
#endif
 NSArray *apps=[NSRunningApplication runningApplicationsWithBundleIdentifier:b];
 if(apps.count!=1)return nil;
 NSRunningApplication *a=apps.firstObject; return a.terminated||!a.launchDate ? nil:a;
}
static NSString *launch(NSRunningApplication *a) {return [NSString stringWithFormat:@"%.6f",a.launchDate.timeIntervalSince1970];}
static NSAppleEventDescriptor *address(NSRunningApplication *a) {pid_t pid=a.processIdentifier;return [NSAppleEventDescriptor descriptorWithDescriptorType:typeKernelProcessID bytes:&pid length:sizeof(pid)];}
static BOOL same(NSString *p,int pid,NSString *born) {NSRunningApplication *a=running(p);return a&&a.processIdentifier==pid&&[launch(a)isEqual:born];}
static OSStatus permission(NSRunningApplication *a,BOOL ask) {
#ifdef OT_MEDIA_FIXTURE
 return ot_fixture_permission(a,ask);
#endif
 return AEDeterminePermissionToAutomateTarget(address(a).aeDesc,typeWildCard,typeWildCard,ask);}
static NSDictionary *permissionResult(OSStatus status) {
 if(status==noErr)return failure(@"ready",@"");
 if(status==procNotFound)return failure(@"notRunning",@"Player is not running");
 if(status==errAEEventWouldRequireUserConsent)return failure(@"permissionRequired",@"Connect this player to allow Automation access");
 if(status==errAEEventNotPermitted)return failure(@"denied",@"Automation access is denied");
 return failure(@"unavailable",[NSString stringWithFormat:@"Automation error %d",(int)status]);
}
static NSAppleEventDescriptor *property(OSType code,NSAppleEventDescriptor *container){
 NSAppleEventDescriptor *r=[NSAppleEventDescriptor recordDescriptor];
 [r setDescriptor:[NSAppleEventDescriptor descriptorWithTypeCode:typeProperty] forKeyword:keyAEDesiredClass];
 [r setDescriptor:container?:[NSAppleEventDescriptor nullDescriptor] forKeyword:keyAEContainer];
 [r setDescriptor:[NSAppleEventDescriptor descriptorWithEnumCode:formPropertyID] forKeyword:keyAEKeyForm];
 [r setDescriptor:[NSAppleEventDescriptor descriptorWithTypeCode:code] forKeyword:keyAEKeyData];
 return [r coerceToDescriptorType:typeObjectSpecifier];
}
static NSAppleEventDescriptor *mediaSend(NSRunningApplication *a,OSType suite,OSType code,NSAppleEventDescriptor *object,NSAppleEventDescriptor *value,double deadline,NSError **error){
 double remaining=deadline-NSProcessInfo.processInfo.systemUptime;
 if(remaining<=0){if(error)*error=[NSError errorWithDomain:@"OptionTabMedia" code:errAETimeout userInfo:nil];return nil;}
 NSAppleEventDescriptor *e=[NSAppleEventDescriptor appleEventWithEventClass:suite eventID:code targetDescriptor:address(a) returnID:kAutoGenerateReturnID transactionID:kAnyTransactionID];
 if(object)[e setParamDescriptor:object forKeyword:keyDirectObject];if(value)[e setParamDescriptor:value forKeyword:keyAEData];
 NSAppleEventDescriptor *reply;
#ifdef OT_MEDIA_FIXTURE
 reply=ot_fixture_send(e,remaining,error);
#else
 reply=[e sendEventWithOptions:kAEWaitReply|kAENeverInteract|kAEDontRecord timeout:remaining error:error];
#endif
 NSAppleEventDescriptor *failureCode=[reply paramDescriptorForKeyword:keyErrorNumber];
 if(failureCode.int32Value!=0){if(error)*error=[NSError errorWithDomain:@"OptionTabMedia" code:failureCode.int32Value userInfo:nil];return nil;}
 return [reply paramDescriptorForKeyword:keyDirectObject]?:reply;
}
static NSAppleEventDescriptor *get(NSRunningApplication *a,OSType code,BOOL track,double deadline,NSError **error){return mediaSend(a,kAECoreSuite,kAEGetData,property(code,track?property('pTrk',nil):nil),nil,deadline,error);}
static NSString *text(NSAppleEventDescriptor *d){
 if(!d)return nil;DescType t=d.descriptorType;
 if(t!=typeUnicodeText&&t!=typeUTF8Text&&t!=typeChar&&t!=typeCString)return nil;
 NSString *s=d.stringValue;return [s lengthOfBytesUsingEncoding:NSUTF8StringEncoding]<=4096?s:nil;
}
static BOOL number(NSAppleEventDescriptor *d,double *value){
 if(!d)return NO;DescType t=d.descriptorType;
 if(t!=typeSInt16&&t!=typeSInt32&&t!=typeSInt64&&t!=typeIEEE32BitFloatingPoint&&t!=typeIEEE64BitFloatingPoint)return NO;
 NSAppleEventDescriptor *coerced=[d coerceToDescriptorType:typeIEEE64BitFloatingPoint];NSData *data=coerced.data;if(data.length!=sizeof(double))return NO;
 memcpy(value,data.bytes,sizeof(double));return isfinite(*value)&&*value>=0&&*value<=86400000;
}
static NSString *trackID(NSRunningApplication *a,NSString *p,double deadline,NSError **error){return text(get(a,[p isEqual:@"music"]?'pPIS':'ID  ',YES,deadline,error));}
char *ot_media_permission(const char *provider,int ask){@autoreleasepool{
 NSString *p=@(provider);if(!bundle(p))return json(failure(@"unsupported",@"Unsupported player"));
 NSRunningApplication *a=running(p);return json(a?permissionResult(permission(a,ask)):failure(@"notRunning",@"Player is not running"));
}}
char *ot_media_read(const char *provider){@autoreleasepool{
 NSString *p=@(provider);NSRunningApplication *a=running(p);if(!a)return json(failure(@"notRunning",@"Player is not running"));
 OSStatus access=permission(a,NO);if(access)return json(permissionResult(access));
 NSString *born=launch(a);double deadline=NSProcessInfo.processInfo.systemUptime+2;NSError *error=nil;
 NSString *identity=trackID(a,p,deadline,&error);
 NSString *title=text(get(a,'pnam',YES,deadline,&error));NSString *artist=text(get(a,'pArt',YES,deadline,&error));NSString *album=text(get(a,'pAlb',YES,deadline,&error));
 NSAppleEventDescriptor *state=get(a,'pPlS',NO,deadline,&error);double position=0,duration=0;BOOL positionOK=number(get(a,'pPos',NO,deadline,&error),&position);
 BOOL music=[p isEqual:@"music"];BOOL durationOK=music&&number(get(a,'pDur',YES,deadline,&error),&duration);
 NSString *art=music?@"music-artwork":text(get(a,'aUrl',YES,deadline,&error));
 NSString *last=trackID(a,p,deadline,&error);
 if(!same(p,a.processIdentifier,born))return json(failure(@"notRunning",@"Player identity changed"));
 if(error||!identity.length||![identity isEqual:last]||!title||!artist||!album||!positionOK||state.descriptorType!=typeEnumerated)return json(failure(@"unavailable",error.localizedDescription?:@"Track metadata unavailable or changed"));
 OSType playback=state.enumCodeValue;NSString *playing=playback=='kPSP'?@"playing":playback=='kPSp'?@"paused":playback=='kPSS'?@"stopped":@"unknown";
 return json(@{@"status":@"ready",@"reason":music?@"":@"Spotify duration and seek are not verified",@"process":@{@"pid":@(a.processIdentifier),@"launchID":born},@"track":@{@"id":identity,@"title":title,@"artist":artist,@"album":album,@"durationMS":@(durationOK?(int64_t)(duration*1000):0)},@"playback":playing,@"positionMS":@((int64_t)(position*1000)),@"capabilities":@{@"play":@YES,@"pause":@YES,@"previous":@YES,@"next":@YES,@"seek":@(durationOK&&duration>0)},@"artwork":art?:@""});
}}
char *ot_media_command(const char *provider,int pid,const char *birth,const char *track,const char *kind,int64_t position,uintptr_t guard){@autoreleasepool{
 NSString *p=@(provider),*born=@(birth),*identity=@(track),*command=@(kind);if(!same(p,pid,born))return strdup("Player identity changed");
 NSRunningApplication *a=running(p);if(permission(a,NO))return strdup("Automation access unavailable");
 double deadline=NSProcessInfo.processInfo.systemUptime+2;NSError *error=nil;NSString *current=trackID(a,p,deadline,&error);
 if(error||![identity isEqual:current])return strdup("Media track changed");
 OSType code=0;NSAppleEventDescriptor *object=nil,*value=nil;
 if([command isEqual:@"play"])code='Play';else if([command isEqual:@"pause"])code='Paus';else if([command isEqual:@"next"])code='Next';else if([command isEqual:@"previous"])code='Prev';
 else if([command isEqual:@"seek"]&&[p isEqual:@"music"]){double duration=0;if(!number(get(a,'pDur',YES,deadline,&error),&duration)||position<0||position/1000.0>=duration)return strdup("Seek position unavailable");object=property('pPos',nil);double seconds=position/1000.0;value=[NSAppleEventDescriptor descriptorWithDescriptorType:typeIEEE64BitFloatingPoint bytes:&seconds length:sizeof(seconds)];code=kAESetData;}
 if(!code)return strdup("Unsupported media command");
 if(![trackID(a,p,deadline,&error)isEqual:identity]||error)return strdup("Media track changed");
 if(!same(p,pid,born)||!ot_media_guard(guard))return strdup("Media command retired");
 mediaSend(a,value?kAECoreSuite:([p isEqual:@"music"]?'hook':'spfy'),code,object,value,deadline,&error);
 return error?strdup(error.localizedDescription.UTF8String):NULL;
}}
char *ot_media_artwork(const char *provider,int pid,const char *birth,const char *track){@autoreleasepool{
 NSString *p=@(provider);if(![p isEqual:@"music"]||!same(p,pid,@(birth)))return NULL;
 NSRunningApplication *a=running(p);if(permission(a,NO))return NULL;
 double deadline=NSProcessInfo.processInfo.systemUptime+2;NSError *error=nil;if(![trackID(a,p,deadline,&error)isEqual:@(track)])return NULL;
 NSAppleEventDescriptor *r=[NSAppleEventDescriptor recordDescriptor];[r setDescriptor:[NSAppleEventDescriptor descriptorWithTypeCode:'cArt'] forKeyword:keyAEDesiredClass];[r setDescriptor:property('pTrk',nil) forKeyword:keyAEContainer];[r setDescriptor:[NSAppleEventDescriptor descriptorWithEnumCode:formAbsolutePosition] forKeyword:keyAEKeyForm];[r setDescriptor:[NSAppleEventDescriptor descriptorWithInt32:1] forKeyword:keyAEKeyData];
 NSAppleEventDescriptor *d=mediaSend(a,kAECoreSuite,kAEGetData,property('pRaw',[r coerceToDescriptorType:typeObjectSpecifier]),nil,deadline,&error);NSData *data=d.data;
 if(error||data.length>5*1024*1024||!same(p,pid,@(birth))||![trackID(a,p,deadline,&error)isEqual:@(track)])return NULL;
 return strdup([data base64EncodedStringWithOptions:0].UTF8String);
}}
