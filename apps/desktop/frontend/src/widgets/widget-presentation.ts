// Keys are issued by the host after checking exact built-in package identity,
// or when creating an unnamed-device fallback. Literal content stays literal.
export function widgetText(
  text: string,
  key: string | undefined,
  t: (text: string) => string,
): string {
  switch (key) {
    case "battery.charging":
      return t("Charging");
    case "battery.notCharging":
      return t("Not charging");
    case "battery.power":
      return t("Battery power");
    case "battery.external":
      return t("External power");
    case "battery.unknown":
      return t("Unknown power source");
    case "network.connected":
      return t("Connected");
    case "network.disconnected":
      return t("Disconnected");
    case "network.none":
      return t("No connection");
    case "network.wifi":
      return t("Wi-Fi");
    case "network.ethernet":
      return t("Ethernet");
    case "network.vpn":
      return t("VPN");
    case "network.other":
      return t("Other network");
    case "network.unknown":
      return t("Unknown network");
    case "audio.muted":
      return t("Muted");
    case "audio.notMuted":
      return t("Not muted");
    case "audio.chooseOutput":
      return t("Choose output");
    case "audio.output":
      return t("Audio output");
    default:
      return text;
  }
}

export function widgetActionError(reason: unknown): "busy" | "retired" | "unavailable" {
  const message =
    reason instanceof Error ? reason.message : typeof reason === "string" ? reason : "";
  if (message === "widgets: action busy") return "busy";
  if (message === "widgets: retired lease") return "retired";
  return "unavailable";
}

export function widgetActionErrorText(error: string, t: (text: string) => string): string {
  if (error === "busy") return t("Widget action in progress");
  if (error === "retired") return t("This widget action is no longer available. Open it again.");
  return t("Action unavailable");
}
