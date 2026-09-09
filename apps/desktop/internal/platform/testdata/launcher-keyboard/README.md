# Isolated launcher keyboard policy fixture

Run from apps/desktop:

    go test -race ./internal/platform -run TestLauncherKeyboard -count=1

The standalone Objective-C fixture uses an NSObject-based fake panel and an unattached NSView. It creates no window, posts no keyboard/input events, activates no application, and changes no desktop state. It covers explicit permission, refused final guard, Escape policy extraction, blur, stale admission, hide, close, physical-scope refusal and successor preservation. Existing native launcher/wheel/material fixtures verify the default panel remains non-key.

This does not prove actual WKWebView committed-text delivery, IME behavior, nonactivating focus acquisition, or focus return to another application. Those require separately coordinated real native acceptance. Production never activates another app to restore focus. Native Escape consumes the key and resigns permission; UI must treat blur/failed current-key validation as retirement and refresh its mode state.
