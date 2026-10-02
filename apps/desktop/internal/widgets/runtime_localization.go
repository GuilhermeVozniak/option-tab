package widgets

// These presentation keys belong to the host, never the manifest. Call only
// for an exact built-in digest and a present, admitted provider reading.
func builtinReadingKey(binding Binding, value Value) string {
	switch binding.Provider + "." + binding.Field {
	case "battery.charging":
		if *value.Boolean {
			return "battery.charging"
		}
		return "battery.notCharging"
	case "battery.powerSource":
		switch *value.Text {
		case "battery":
			return "battery.power"
		case "external":
			return "battery.external"
		case "unknown":
			return "battery.unknown"
		}
	case "network.connected":
		if *value.Boolean {
			return "network.connected"
		}
		return "network.disconnected"
	case "network.category":
		switch *value.Text {
		case "none", "wifi", "ethernet", "vpn", "other", "unknown":
			return "network." + *value.Text
		}
	case "audio.muted":
		if *value.Boolean {
			return "audio.muted"
		}
		return "audio.notMuted"
	}
	return ""
}
