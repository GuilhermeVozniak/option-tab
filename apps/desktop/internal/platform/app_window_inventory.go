package platform

func applicationWindowPresence(identityValid, axReadable bool, axWindows, cgCandidates int) WindowPresence {
	if !identityValid || !axReadable || axWindows < 0 {
		return WindowsUnknown
	}
	if axWindows > 0 {
		return WindowsPresent
	}
	// AXWindows can omit other Spaces. CG's all-windows list corroborates
	// absence; any unresolved surface (including a menu replica) is unknown.
	if cgCandidates == 0 {
		return WindowsNone
	}
	return WindowsUnknown
}
