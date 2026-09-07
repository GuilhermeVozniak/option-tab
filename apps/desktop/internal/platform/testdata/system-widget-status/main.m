#define getifaddrs widgetFixtureAddresses
#define freeifaddrs widgetFixtureFree
#import "../../darwin_system_widget_status.m"
#undef getifaddrs
#undef freeifaddrs

static struct if_data fixtureData;
static struct sockaddr_dl fixtureLink;
static struct ifaddrs fixtureAddress;
int widgetFixtureAddresses(struct ifaddrs **out) {
  *out = &fixtureAddress;
  return 0;
}
void widgetFixtureFree(struct ifaddrs *list) {}
static NSDictionary *fixtureCounter(void) {
  char *raw = ot_widget_network_counters("fixture", 12);
  NSData *data = [[NSString stringWithUTF8String:raw]
      dataUsingEncoding:NSUTF8StringEncoding];
  free(raw);
  return [NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];
}
int main(void) {
  @autoreleasepool {
    NSDictionary *absent = widgetBattery(@[]);
    NSCAssert([absent[@"Status"] isEqual:@"absent"] && !absent[@"Charge"],
              @"desktop fabricated zero battery");
    NSCAssert([widgetBattery(@[ @{} ])[@"Status"] isEqual:@"unavailable"],
              @"malformed source classified as absent");
    NSMutableDictionary *battery = [@{
      @kIOPSTypeKey : @kIOPSInternalBatteryType,
      @kIOPSCurrentCapacityKey : @50,
      @kIOPSMaxCapacityKey : @100,
      @kIOPSIsChargingKey : @YES,
      @kIOPSPowerSourceStateKey : @kIOPSACPowerValue
    } mutableCopy];
    NSDictionary *valid = widgetBattery(@[ battery ]);
    NSCAssert([valid[@"Charge"] doubleValue] == .5 &&
                  [valid[@"PowerSource"] isEqual:@"external"] &&
                  [valid[@"Charging"] boolValue],
              @"battery normalization");
    battery[@kIOPSMaxCapacityKey] = @0;
    NSCAssert(!widgetBattery(@[ battery ])[@"Charge"],
              @"invalid capacity accepted");
    battery[@kIOPSMaxCapacityKey] = @YES;
    NSCAssert(!widgetBattery(@[ battery ])[@"Charge"],
              @"boolean capacity accepted");
    NSCAssert(
        [widgetCategory(kSCNetworkInterfaceTypeIEEE80211) isEqual:@"wifi"] &&
            [widgetCategory(kSCNetworkInterfaceTypeEthernet)
                isEqual:@"ethernet"] &&
            [widgetCategory(kSCNetworkInterfaceTypeIPSec) isEqual:@"vpn"],
        @"public interface categories");
    NSDictionary *off = widgetNetwork(nil, YES, NO, 0, 0, nil);
    NSCAssert(![off[@"Connected"] boolValue] &&
                  [off[@"Category"] isEqual:@"none"],
              @"no route fabricated connection");
    NSCAssert(!widgetNetwork(@"iface", NO, YES, IFF_UP | IFF_RUNNING, 1,
                             @"wifi")[@"Connected"],
              @"uncertain route fabricated known disconnected");
    NSCAssert([widgetNetwork(@"iface", YES, YES, IFF_UP | IFF_RUNNING, 1,
                             @"wifi")[@"Connected"] boolValue],
              @"active local route refused");
    NSCAssert(![widgetNetwork(@"iface", YES, YES, IFF_UP, 1,
                              @"wifi")[@"Connected"] boolValue],
              @"non-running link connected");
    fixtureLink.sdl_family = AF_LINK;
    fixtureLink.sdl_index = 12;
    fixtureAddress.ifa_name = "fixture";
    fixtureAddress.ifa_addr = (struct sockaddr *)&fixtureLink;
    fixtureAddress.ifa_flags = IFF_UP | IFF_RUNNING;
    fixtureAddress.ifa_data = &fixtureData;
    fixtureData.ifi_type = 6;
    fixtureData.ifi_ibytes = 100;
    fixtureData.ifi_obytes = 200;
    fixtureData.ifi_lastchange.tv_sec = 1000;
    NSDictionary *first = fixtureCounter();
    fixtureData.ifi_lastchange.tv_sec++;
    fixtureData.ifi_ibytes += 10;
    fixtureData.ifi_obytes += 20;
    NSDictionary *second = fixtureCounter();
    NSCAssert([first[@"Valid"] boolValue] &&
                  [second[@"Valid"] boolValue] &&
                  [first[@"Identity"] isEqual:second[@"Identity"]] &&
                  [second[@"Upload"] unsignedLongLongValue] == 220 &&
                  [second[@"Download"] unsignedLongLongValue] == 110,
              @"volatile administrative timestamp retired stable counters");
    fixtureLink.sdl_index++;
    NSCAssert(![fixtureCounter()[@"Valid"] boolValue],
              @"replaced interface index accepted");
    puts("PASS injected power dictionaries and route/link/type/counter normalization; "
         "no native power/network read invoked");
  }
  return 0;
}
