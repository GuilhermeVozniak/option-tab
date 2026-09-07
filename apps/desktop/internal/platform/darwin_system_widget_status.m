//go:build darwin
#import "darwin_system_widget_status.h"
#import <Foundation/Foundation.h>
#import <IOKit/ps/IOPSKeys.h>
#import <IOKit/ps/IOPowerSources.h>
#import <SystemConfiguration/SystemConfiguration.h>
#import <ifaddrs.h>
#import <net/if.h>
#import <net/if_dl.h>
static char *widgetJSON(id value) {
  NSData *data = value ? [NSJSONSerialization dataWithJSONObject:value
                                                         options:0
                                                           error:NULL]
                       : nil;
  return data ? strdup([[NSString alloc] initWithData:data
                                             encoding:NSUTF8StringEncoding]
                           .UTF8String)
              : NULL;
}
static NSDictionary *widgetBattery(NSArray *sources) {
  NSMutableDictionary *out = [@{
    @"Status" : @"unavailable",
    @"Reason" : @"powerUnavailable",
    @"PowerSource" : @"unknown"
  } mutableCopy];
  if (![sources isKindOfClass:NSArray.class] || sources.count > 16)
    return out;
  NSMutableArray *batteries = [NSMutableArray array];
  for (id item in sources) {
    if (![item isKindOfClass:NSDictionary.class] ||
        ![item[@kIOPSTypeKey] isKindOfClass:NSString.class] ||
        ![@[ @kIOPSInternalBatteryType, @kIOPSUPSType ]
            containsObject:item[@kIOPSTypeKey]])
      return out;
    if ([item[@kIOPSTypeKey] isEqual:@kIOPSInternalBatteryType])
      [batteries addObject:item];
  }
  if (!batteries.count) {
    out[@"Status"] = @"absent";
    out[@"Reason"] = @"noBattery";
    return out;
  }
  if (batteries.count != 1)
    return out;
  NSDictionary *battery = batteries.firstObject;
  id current = battery[@kIOPSCurrentCapacityKey],
     maximum = battery[@kIOPSMaxCapacityKey];
  for (id n in @[ current ?: NSNull.null, maximum ?: NSNull.null ])
    if (![n isKindOfClass:NSNumber.class] ||
        CFGetTypeID((__bridge CFTypeRef)n) == CFBooleanGetTypeID() ||
        !isfinite([n doubleValue]))
      return out;
  double capacity = [maximum doubleValue], charge = [current doubleValue];
  if (capacity <= 0 || charge < 0 || charge > capacity)
    return out;
  out[@"Charge"] = @(charge / capacity);
  id charging = battery[@kIOPSIsChargingKey];
  if (charging &&
      CFGetTypeID((__bridge CFTypeRef)charging) == CFBooleanGetTypeID())
    out[@"Charging"] = charging;
  id power = battery[@kIOPSPowerSourceStateKey];
  if ([power isEqual:@kIOPSACPowerValue])
    out[@"PowerSource"] = @"external";
  else if ([power isEqual:@kIOPSBatteryPowerValue])
    out[@"PowerSource"] = @"battery";
  out[@"Status"] = @"ready";
  out[@"Reason"] = @"";
  return out;
}
char *ot_widget_battery_read(void) {
  @autoreleasepool {
    CFTypeRef blob = IOPSCopyPowerSourcesInfo();
    if (!blob)
      return widgetJSON(widgetBattery(nil));
    CFArrayRef list = IOPSCopyPowerSourcesList(blob);
    NSMutableArray *sources = [NSMutableArray array];
    BOOL valid = list && CFArrayGetCount(list) <= 16;
    if (valid)
      for (CFIndex i = 0; i < CFArrayGetCount(list); i++) {
        CFDictionaryRef item = IOPSGetPowerSourceDescription(
            blob, CFArrayGetValueAtIndex(list, i));
        if (!item) {
          valid = NO;
          break;
        }
        [sources addObject:(__bridge NSDictionary *)item];
      }
    NSDictionary *result = widgetBattery(valid ? sources : nil);
    if (list)
      CFRelease(list);
    CFRelease(blob);
    return widgetJSON(result);
  }
}
static NSString *widgetPrimary(SCDynamicStoreRef store, CFStringRef entity,
                               BOOL *valid) {
  CFStringRef key = SCDynamicStoreKeyCreateNetworkGlobalEntity(
      NULL, kSCDynamicStoreDomainState, entity);
  id item = CFBridgingRelease(SCDynamicStoreCopyValue(store, key));
  CFRelease(key);
  if (!item) {
    if (SCError() != kSCStatusNoKey)
      *valid = NO;
    return nil;
  }
  if (![item isKindOfClass:NSDictionary.class]) {
    *valid = NO;
    return nil;
  }
  id name = item[@"PrimaryInterface"];
  if (![name isKindOfClass:NSString.class] || ![name length] ||
      [name length] >= IFNAMSIZ) {
    *valid = NO;
    return nil;
  }
  return name;
}
static NSString *widgetCategory(CFStringRef type) {
  if (!type)
    return @"other";
  if (CFEqual(type, kSCNetworkInterfaceTypeIEEE80211))
    return @"wifi";
  if (CFEqual(type, kSCNetworkInterfaceTypeEthernet))
    return @"ethernet";
  if (CFEqual(type, kSCNetworkInterfaceTypeIPSec) ||
      CFEqual(type, kSCNetworkInterfaceTypePPP) ||
      CFEqual(type, kSCNetworkInterfaceTypeL2TP))
    return @"vpn";
  return @"other";
}
static NSDictionary *widgetNetwork(NSString *primary, BOOL routeValid,
                                   BOOL found, unsigned flags, unsigned index,
                                   NSString *category) {
  if (!routeValid)
    return @{
      @"Status" : @"unavailable",
      @"Reason" : @"routeUnavailable",
      @"Category" : @"unknown"
    };
  if (!primary)
    return @{
      @"Status" : @"ready",
      @"Reason" : @"",
      @"Connected" : @NO,
      @"Category" : @"none"
    };
  if (!found || !index)
    return @{
      @"Status" : @"unavailable",
      @"Reason" : @"linkUnavailable",
      @"Category" : @"unknown"
    };
  BOOL connected =
      (flags & IFF_UP) && (flags & IFF_RUNNING) && !(flags & IFF_LOOPBACK);
  return @{
    @"Status" : @"ready",
    @"Reason" : @"",
    @"Connected" : @(connected),
    @"Category" : connected ? category : @"none",
    @"Interface" : primary,
    @"Index" : @(index)
  };
}
char *ot_widget_network_status(void) {
  @autoreleasepool {
    SCDynamicStoreRef store = SCDynamicStoreCreate(
        NULL, CFSTR("org.optiontab.widget-network"), NULL, NULL);
    if (!store)
      return widgetJSON(widgetNetwork(nil, NO, NO, 0, 0, nil));
    BOOL valid = YES;
    NSString *v4 = widgetPrimary(store, kSCEntNetIPv4, &valid),
             *v6 = widgetPrimary(store, kSCEntNetIPv6, &valid);
    CFRelease(store);
    if (v4 && v6 && ![v4 isEqual:v6])
      valid = NO;
    NSString *primary = v4 ?: v6;
    if (!valid || !primary)
      return widgetJSON(widgetNetwork(primary, valid, NO, 0, 0, nil));
    struct ifaddrs *list = NULL;
    if (getifaddrs(&list) != 0)
      return widgetJSON(widgetNetwork(primary, NO, NO, 0, 0, nil));
    BOOL found = NO;
    unsigned flags = 0, index = 0;
    int matches = 0;
    for (struct ifaddrs *p = list; p; p = p->ifa_next) {
      if (!p->ifa_name || !p->ifa_addr || p->ifa_addr->sa_family != AF_LINK ||
          strcmp(p->ifa_name, primary.UTF8String))
        continue;
      matches++;
      struct sockaddr_dl *link = (struct sockaddr_dl *)p->ifa_addr;
      flags = p->ifa_flags;
      index = link->sdl_index;
      found = YES;
    }
    freeifaddrs(list);
    if (matches != 1)
      found = NO;
    NSString *category = @"other";
    CFArrayRef interfaces = SCNetworkInterfaceCopyAll();
    if (interfaces && CFArrayGetCount(interfaces) <= 128)
      for (CFIndex i = 0; i < CFArrayGetCount(interfaces); i++) {
        SCNetworkInterfaceRef iface =
            (SCNetworkInterfaceRef)CFArrayGetValueAtIndex(interfaces, i);
        CFStringRef name = SCNetworkInterfaceGetBSDName(iface);
        if (name && [primary isEqual:(__bridge NSString *)name])
          category = widgetCategory(SCNetworkInterfaceGetInterfaceType(iface));
      }
    if (interfaces)
      CFRelease(interfaces);
    return widgetJSON(
        widgetNetwork(primary, valid, found, flags, index, category));
  }
}
char *ot_widget_network_counters(const char *name, uint32_t expected) {
  @autoreleasepool {
    if (!name || !name[0] || strlen(name) >= IFNAMSIZ || !expected)
      return widgetJSON(@{@"Valid" : @NO});
    struct ifaddrs *list = NULL;
    if (getifaddrs(&list) != 0)
      return widgetJSON(@{@"Valid" : @NO});
    NSDictionary *result = nil;
    int count = 0;
    for (struct ifaddrs *p = list; p; p = p->ifa_next) {
      if (!p->ifa_name || strcmp(p->ifa_name, name) || !p->ifa_addr ||
          p->ifa_addr->sa_family != AF_LINK)
        continue;
      count++;
      struct sockaddr_dl *link = (struct sockaddr_dl *)p->ifa_addr;
      if (link->sdl_index != expected || !p->ifa_data ||
          !(p->ifa_flags & IFF_UP) || !(p->ifa_flags & IFF_RUNNING))
        continue;
      struct if_data *data = p->ifa_data;
      // Some drivers advance ifi_lastchange while the same link remains up.
      // Treat identity separately from that administrative timestamp; the
      // reducer already drops rates on link changes, resets and invalid deltas.
      NSString *identity =
          [NSString stringWithFormat:@"%s:%u:%u", name, expected,
                                     (unsigned)data->ifi_type];
      result = @{
        @"Valid" : @YES,
        @"Identity" : identity,
        @"Upload" : @(data->ifi_obytes),
        @"Download" : @(data->ifi_ibytes)
      };
    }
    freeifaddrs(list);
    return widgetJSON(count == 1 && result ? result : @{@"Valid" : @NO});
  }
}
