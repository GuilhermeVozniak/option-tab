//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type hit struct {
	Target               uint64
	Width, Height, Scale float64
}
type Probe struct {
	ready chan struct{}
	hits  chan hit
}

func (p *Probe) Ready() {
	select {
	case p.ready <- struct{}{}:
	default:
	}
}
func (p *Probe) Hit(target uint64, w, h, scale float64) { p.hits <- hit{target, w, h, scale} }
func main() {
	var originalX, originalY C.double
	C.smoke_pointer(&originalX, &originalY)
	restoreCursor := func() { C.smoke_warp(originalX, originalY) }
	defer restoreCursor()
	pid, _ := strconv.Atoi(os.Getenv("DOCK_PANEL_FIXTURE_PID"))
	if pid <= 0 {
		panic("fixture PID required")
	}
	probe := &Probe{ready: make(chan struct{}, 1), hits: make(chan hit, 8)}
	app := application.New(application.Options{Name: "Option Tab isolated panel smoke", Services: []application.Service{application.NewService(probe)}, Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory}, Assets: application.AssetOptions{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(html)) })}})
	host := app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "hidden-dock-smoke-host", Title: "Hidden Dock smoke host", Width: 300, Height: 160, Hidden: true, Frameless: true})
	app.Event.OnApplicationEvent(events.Mac.ApplicationDidFinishLaunching, func(*application.ApplicationEvent) {
		go func() {
			fail := func(err any) { fmt.Fprintln(os.Stderr, "SMOKE FAIL:", err); os.Exit(2) }
			time.Sleep(600 * time.Millisecond)
			index, _ := strconv.Atoi(os.Getenv("DOCK_PANEL_SCREEN_INDEX"))
			var sx, sy C.double
			if C.smoke_screen(C.int(index), &sx, &sy) == 0 {
				fail("screen unavailable")
			}
			x, y := float64(sx), float64(sy)
			p, _ := platform.New()
			panel, err := p.(platform.DockPanelHost).CreateDockPanel(host.NativeWindow())
			if err != nil {
				fail(err)
			}
			defer panel.Close()
			if C.smoke_activate(C.int(pid)) == 0 {
				fail("fixture activation failed")
			}
			time.Sleep(300 * time.Millisecond)
			if C.smoke_position_fixture(C.int(pid), sx+80, sy+100) == 0 {
				fail("fixture placement failed")
			}
			C.smoke_warp(sx+150, sy+150)
			C.smoke_activate(C.int(pid))
			time.Sleep(300 * time.Millisecond)
			before := int(C.smoke_foreground())
			focus := uint32(C.smoke_focused_window(C.int(pid)))
			if before != pid || focus == 0 {
				fail(fmt.Sprintf("fixture not focused: pid=%d window=%d", before, focus))
			}
			fmt.Printf("FOCUS beforeShow=%d self=%d\n", int(C.smoke_foreground()), os.Getpid())
			if err := panel.Show(domain.Bounds{X: x + 200, Y: y + 200, W: 300, H: 160}); err != nil {
				fail(err)
			}
			select {
			case <-probe.ready:
			case <-time.After(8 * time.Second):
				fail("Wails runtime did not become ready")
			}
			fmt.Printf("FOCUS beforeExecJS=%d\n", int(C.smoke_foreground()))
			host.ExecJS(fmt.Sprintf("window.smokeTarget=%d", focus))
			fmt.Printf("FOCUS afterExecJS=%d\n", int(C.smoke_foreground()))
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("FOCUS beforeClick=%d\n", int(C.smoke_foreground()))
			if int(C.smoke_foreground()) != pid {
				fail("external foreground change before panel click")
			}
			C.smoke_click(sx+260, sy+250)
			var first hit
			select {
			case first = <-probe.hits:
			case <-time.After(3 * time.Second):
				fail("actual panel click did not invoke Wails bridge")
			}
			if first.Target != uint64(focus) || first.Width != 300 || first.Height != 160 {
				fail(fmt.Sprintf("unexpected callback/viewport %+v", first))
			}
			after := int(C.smoke_foreground())
			afterFocus := uint32(C.smoke_focused_window(C.int(pid)))
			if after != before || afterFocus != focus {
				fail(fmt.Sprintf("panel stole focus: before %d/%d after %d/%d", before, focus, after, afterFocus))
			}
			C.smoke_key()
			time.Sleep(200 * time.Millisecond)
			path := os.Getenv("DOCK_PANEL_FIXTURE_LOG")
			log, _ := os.ReadFile(path)
			if !strings.Contains(string(log), "fixture-key-x") {
				fail("typing did not reach foreground text fixture")
			}
			if strings.Contains(string(log), "fixture-mouse") {
				fail("panel click leaked to fixture")
			}
			windows, _ := p.Windows()
			for _, w := range windows {
				if uint32(w.ID) == focus {
					fmt.Printf("FIXTURE bounds=%+v screenOrigin=%g,%g\n", w.Bounds, x, y)
				}
			}
			if int(C.smoke_foreground()) != pid {
				fail("external foreground change before outside click")
			}
			C.smoke_click(sx+195, sy+250)
			time.Sleep(200 * time.Millisecond)
			log, _ = os.ReadFile(path)
			if !strings.Contains(string(log), "fixture-mouse") {
				fail(fmt.Sprintf("outside panel click did not reach fixture; log=%s", log))
			}
			native := C.smoke_state(host.NativeWindow())
			fmt.Printf("NATIVE %s\n", C.GoString(native))
			C.free(unsafe.Pointer(native))
			fmt.Printf("BRIDGE target=%d viewport=%gx%g DPR=%g foreground=%d->%d focusedWindow=%d->%d outsideClick=fixture typed=x\n", first.Target, first.Width, first.Height, first.Scale, before, after, focus, afterFocus)
			for i := 0; i < 5; i++ {
				if err := panel.Hide(); err != nil {
					fail(err)
				}
				if err := panel.Show(domain.Bounds{X: x + 200, Y: y + 200, W: 320 + float64(i)*10, H: 180}); err != nil {
					fail(err)
				}
			}
			if err := panel.Close(); err != nil {
				fail(err)
			}
			if err := panel.Close(); err != nil {
				fail(err)
			}
			if panel.Show(domain.Bounds{W: 100, H: 100}) == nil {
				fail("closed panel revived")
			}
			time.Sleep(300 * time.Millisecond)
			native = C.smoke_state(host.NativeWindow())
			fmt.Printf("CLOSED %s\n", C.GoString(native))
			C.free(unsafe.Pointer(native))

			if _, err := p.(platform.DockPanelHost).CreateDockPanel(nil); err == nil {
				fail("nil host accepted")
			}
			recreated, err := p.(platform.DockPanelHost).CreateDockPanel(host.NativeWindow())
			if err != nil {
				fail(fmt.Sprintf("restore/recreate: %v", err))
			}
			if err := recreated.Show(domain.Bounds{X: x + 200, Y: y + 200, W: 310, H: 170}); err != nil {
				fail(err)
			}
			time.Sleep(250 * time.Millisecond)
			C.smoke_click(sx+260, sy+250)
			select {
			case got := <-probe.hits:
				if got.Target != uint64(focus) || got.Width != 310 || got.Height != 170 {
					fail(fmt.Sprintf("recreated viewport %+v", got))
				}
			case <-time.After(2 * time.Second):
				fail("recreated Wails bridge unavailable")
			}
			oldHost := host.NativeWindow()
			host.Close()
			time.Sleep(300 * time.Millisecond)
			if err := recreated.Show(domain.Bounds{W: 100, H: 100}); err == nil {
				fail("closed Wails host panel revived")
			}
			_ = recreated.Hide()
			_ = recreated.Close()
			_ = recreated.Close()
			if _, err := p.(platform.DockPanelHost).CreateDockPanel(oldHost); err == nil {
				fail("destroyed host pointer accepted")
			}
			fmt.Println("LIFECYCLE restore/recreate bridge=pass hostClose=terminal stalePointer=rejected")
			fmt.Println("SMOKE PASS")
			app.Quit()
		}()
	})
	go func() { time.Sleep(25 * time.Second); fmt.Fprintln(os.Stderr, "SMOKE TIMEOUT"); os.Exit(3) }()
	if err := app.Run(); err != nil {
		panic(err)
	}
}

const html = `<!doctype html><html><head><style>html,body{margin:0;background:#202838;color:white;font:16px system-ui}button{margin:20px;width:220px;height:90px;font-size:20px}</style></head><body><button id="target">Target window 337</button><script type="module">import {Call} from '/wails/runtime.js';document.querySelector('button').onclick=()=>Call.ByName('main.Probe.Hit',window.smokeTarget||337,innerWidth,innerHeight,devicePixelRatio);Call.ByName('main.Probe.Ready');</script></body></html>`
