package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Option Tab",
		Width:  1280,
		Height: 820,
		// The switcher is a frameless, transparent, always-on-top overlay that
		// starts hidden and is shown on the global hotkey.
		Frameless:        true,
		AlwaysOnTop:      true,
		StartHidden:      true,
		DisableResize:    true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind:             []any{app},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			// WindowIsTranslucent is deliberately off: it inserts a full-window
			// NSVisualEffectView (a dark square backdrop). The overlay wants a
			// fully clear window where only the webview's rounded panel is
			// visible; ot_window_init_overlay finishes the job at startup.
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
