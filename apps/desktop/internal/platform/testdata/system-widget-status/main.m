#import "../../darwin_system_widget_status.m"
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
    puts("PASS injected power dictionaries and route/link/type normalization; "
         "no native power/network read invoked");
  }
  return 0;
}
