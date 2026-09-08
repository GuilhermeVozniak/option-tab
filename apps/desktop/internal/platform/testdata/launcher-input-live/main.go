//go:build darwin

// Explicit manual port probe; this package is excluded by normal ./... traversal.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type (
	request struct {
		kind    string
		enabled bool
		done    chan bool
	}
	Probe struct {
		ctx       context.Context
		cancel    context.CancelFunc
		ready     chan struct{}
		jobs      chan request
		bound     atomic.Pointer[platform.LauncherDisplay]
		dock      atomic.Pointer[domain.Bounds]
		bounds    atomic.Pointer[domain.Bounds]
		last      atomic.Int64
		key       atomic.Bool
		committed atomic.Uint64
	}
)

func (p *Probe) Ready() {
	select {
	case p.ready <- struct{}{}:
	default:
	}
}
func (p *Probe) Stop() { p.cancel() }
func (p *Probe) Keyboard(enabled bool) bool {
	return p.request(request{kind: "keyboard", enabled: enabled})
}

func (p *Probe) Committed(text string) bool {
	// No content is retained, echoed or logged. Even committed composition is only counted.
	if len(text) == 0 || len(text) > 256 || !utf8.ValidString(text) {
		return false
	}
	return p.request(request{kind: "committed"})
}

func (p *Probe) request(r request) bool {
	r.done = make(chan bool, 1)
	select {
	case p.jobs <- r:
	case <-p.ctx.Done():
		return false
	default:
		return false
	}
	select {
	case v := <-r.done:
		return v
	case <-p.ctx.Done():
		return false
	}
}

func (p *Probe) current() bool {
	return p.ctx.Err() == nil && p.bound.Load() != nil && time.Since(time.Unix(0, p.last.Load())) < time.Second
}

func main() {
	run := flag.Bool("run-native", false, "explicitly open manual-input probe")
	display := flag.String("display", "", "exact display UUID")
	duration := flag.Duration("duration", 90*time.Second, "lifetime, 15s..180s")
	flag.Parse()
	if !*run || !regexp.MustCompile(`^[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}$`).MatchString(*display) || *duration < 15*time.Second || *duration > 180*time.Second {
		fmt.Fprintln(os.Stderr, "REFUSED: --run-native, --display UUID and duration15s..180s required")
		return
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *duration)
	defer cancel()
	p := &Probe{ctx: ctx, cancel: cancel, ready: make(chan struct{}, 1), jobs: make(chan request, 8)}
	var cleaned atomic.Bool
	app := application.New(application.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Name: "Option Tab manual input port probe", Services: []application.Service{application.NewService(p)}, Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory}, ShouldQuit: func() bool {
		if cleaned.Load() {
			return true
		}
		cancel()
		return false
	}, Assets: application.AssetOptions{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	})}})
	host := app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "launcher-input-port-probe", Title: "Manual input port probe", Width: 420, Height: 260, Hidden: true, Frameless: true})
	app.Event.OnApplicationEvent(events.Mac.ApplicationDidFinishLaunching, func(*application.ApplicationEvent) {
		go func() {
			defer func() { cleaned.Store(true); app.Quit() }()
			native, err := platform.New()
			if err != nil {
				fmt.Println("REFUSED platform")
				return
			}
			source, ok := native.(platform.LauncherEnvironmentSource)
			if !ok {
				fmt.Println("REFUSED environment")
				return
			}
			envReady := make(chan platform.LauncherDisplay, 1)
			envDone := make(chan struct{})
			watchCtx, watchCancel := context.WithCancel(ctx)
			go func() {
				defer close(envDone)
				_ = source.ObserveLauncherEnvironment(watchCtx, func(e platform.LauncherEnvironment) {
					var found *platform.LauncherDisplay
					if e.NativeDock.Confidence == "known" {
						dock := e.NativeDock.Bounds
						p.dock.Store(&dock)
						if surface := p.bounds.Load(); surface != nil && intersects(*surface, dock) {
							cancel()
							return
						}
					}
					if e.Complete && e.Status == "ready" {
						for _, d := range e.Displays {
							if d.UUID == *display && d.SpaceKind == "ordinary" && d.SpaceStatus == "known" && d.SpaceID != 0 {
								copy := d
								found = &copy
								break
							}
						}
					}
					if bound := p.bound.Load(); bound != nil {
						if found == nil || *bound != *found {
							cancel()
							return
						}
						p.last.Store(time.Now().UnixNano())
						return
					}
					if found != nil {
						select {
						case envReady <- *found:
						default:
						}
					}
				})
				cancel()
			}()
			defer func() { watchCancel(); <-envDone; fmt.Println("CLEANUP environmentJoined=true") }()
			var d platform.LauncherDisplay
			select {
			case d = <-envReady:
			case <-ctx.Done():
				fmt.Println("REFUSED ordinary display")
				return
			case <-time.After(5 * time.Second):
				fmt.Println("REFUSED display readiness")
				return
			}
			p.bound.Store(&d)
			p.last.Store(time.Now().UnixNano())
			// An intentionally interior port surface, not the production Dock edge layout.
			u := d.UsableFrame
			b := domain.Bounds{X: u.X + (u.W-420)/2, Y: u.Y + (u.H-260)/2, W: 420, H: 260}
			if b.X < d.Frame.X+64 || b.Y < d.Frame.Y+64 || b.X+b.W > d.Frame.X+d.Frame.W-64 || b.Y+b.H > d.Frame.Y+d.Frame.H-64 {
				fmt.Println("REFUSED safe interior")
				return
			}
			dock := p.dock.Load()
			if dock == nil || intersects(b, *dock) {
				fmt.Println("REFUSED native Dock overlap")
				return
			}
			p.bounds.Store(&b)
			factory, ok := native.(platform.LauncherPanelHost)
			if !ok {
				fmt.Println("REFUSED launcher host")
				return
			}
			panel, err := factory.CreateLauncherPanel(host.NativeWindow(), *display)
			if err != nil {
				fmt.Println("REFUSED panel creation")
				return
			}
			defer func() {
				p.key.Store(false)
				_ = panel.Hide()
				_ = panel.Close()
				if drain, ok := panel.(platform.LauncherGestureDrainer); ok {
					<-drain.LauncherGestureDone()
				}
				host.Close()
				fmt.Println("CLEANUP panelClosed=true gestureJoined=true")
			}()
			gesture, gok := panel.(platform.LauncherGestureSource)
			key, kok := panel.(platform.LauncherKeyboardSource)
			validator, vok := panel.(platform.LauncherPanelValidator)
			keyValidator, kvok := panel.(platform.LauncherKeyboardValidator)
			gestureValidator, gvok := panel.(platform.LauncherGestureValidator)
			ack, aok := panel.(platform.LauncherGestureAcknowledger)
			if !gok || !kok || !vok || !kvok || !gvok || !aok {
				fmt.Println("REFUSED required ports")
				return
			}
			if !p.current() {
				return
			}
			if panel.Show(b) != nil {
				fmt.Println("REFUSED show")
				return
			}
			select {
			case <-p.ready:
			case <-ctx.Done():
				return
			case <-time.After(8 * time.Second):
				fmt.Println("REFUSED Wails ready")
				return
			}
			if validator.ValidateLauncherPanel(ctx, *display) != nil || !p.current() {
				fmt.Println("REFUSED physical panel")
				return
			}
			packets := make(chan platform.LauncherGestureEvent, 64)
			policy := platform.LauncherGesturePolicy{Epoch: 1, Session: 1, Revision: 1, Admission: 1, DisplayUUID: *display, Enabled: true, Scroll: true, Magnify: true, Swipe: true, Bounds: domain.Bounds{W: b.W, H: b.H}}
			if gesture.SetLauncherGesturePolicy(policy, func(e platform.LauncherGestureEvent) {
				select {
				case packets <- e:
				default:
					cancel()
				}
			}) != nil {
				fmt.Println("REFUSED gesture policy")
				return
			}
			fmt.Println("READY portProbe=true noActions=true manualInputOnly=true focusPreservation=unverified")
			var scroll, magnify, swipe uint64
			admission := uint64(1)
			tick := time.NewTicker(250 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					fmt.Printf("SUMMARY scrollPackets=%d magnifyPackets=%d swipePackets=%d committedEvents=%d\n", scroll, magnify, swipe, p.committed.Load())
					return
				case e := <-packets:
					if p.current() && e.Epoch == 1 && e.Session == 1 && e.Revision == 1 && e.Admission == 1 && e.DisplayUUID == *display && e.Owned &&
						gestureValidator.ValidateLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID) &&
						validator.ValidateLauncherPanel(ctx, *display) == nil && p.current() &&
						gestureValidator.ValidateLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID) {
						switch e.Kind {
						case "scroll":
							scroll++
						case "magnify":
							magnify++
						case "swipe":
							swipe++
						}
					}
					ack.CompleteLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID)
				case r := <-p.jobs:
					valid := p.current() && validator.ValidateLauncherPanel(ctx, *display) == nil
					if r.kind == "keyboard" && valid {
						admission++
						v := platform.LauncherKeyboardPolicy{Epoch: 1, Session: 1, Revision: 1, Admission: admission, DisplayUUID: *display, Enabled: r.enabled}
						valid = key.SetLauncherKeyboardPolicy(v, p.current) == nil && p.current()
						if valid && r.enabled {
							valid = keyValidator.ValidateLauncherKeyboard(1, 1, 1, admission)
						}
						p.key.Store(valid && r.enabled)
						fmt.Printf("KEY permission=%t\n", p.key.Load())
						if !valid {
							v.Enabled = false
							_ = key.SetLauncherKeyboardPolicy(v, nil)
						}
					} else if r.kind == "committed" {
						valid = valid && p.key.Load() && keyValidator.ValidateLauncherKeyboard(1, 1, 1, admission) && p.current()
						if valid {
							p.committed.Add(1)
						}
					}
					r.done <- valid
				case <-tick.C:
					if !p.current() || validator.ValidateLauncherPanel(ctx, *display) != nil {
						cancel()
					}
					if p.key.Load() && !keyValidator.ValidateLauncherKeyboard(1, 1, 1, admission) {
						p.key.Store(false)
						fmt.Println("KEY permission=false")
					}
				}
			}
		}()
	})
	if app.Run() != nil {
		fmt.Println("REFUSED Wails runtime")
	}
}

const page = `<!doctype html><meta charset="utf-8"><style>body{margin:20px;font:14px system-ui;background:#182331;color:white}button,input{font:inherit;margin:5px;padding:9px}input{width:330px}p{line-height:1.4}</style><h2>Manual input port probe</h2><p>Scroll, pinch or swipe here manually. No app/window actions are attached. Keyboard input requires the explicit button below. Type only disposable test text.</p><button id="on">Enter keyboard mode</button><button id="off">Leave keyboard mode</button><input id="text" disabled placeholder="Disposable committed text only"><button id="stop">Stop probe</button><span id="status">Loading</span><script type="module">import {Call} from '/wails/runtime.js';const input=document.querySelector('#text'),status=document.querySelector('#status');async function mode(enabled){try{const ok=await Call.ByName('main.Probe.Keyboard',enabled);input.disabled=!enabled||!ok;if(enabled&&ok)input.focus();status.textContent=enabled&&ok?'Keyboard permitted':'Keyboard inactive'}catch{status.textContent='Refused'}}document.querySelector('#on').onclick=()=>mode(true);document.querySelector('#off').onclick=()=>mode(false);document.querySelector('#stop').onclick=()=>Call.ByName('main.Probe.Stop');input.oninput=e=>{if(e.isComposing)return;const text=input.value;input.value='';if(e.inputType==='insertText'&&text)Call.ByName('main.Probe.Committed',text).catch(()=>{})};input.oncompositionend=e=>{const text=e.data;input.value='';if(text)Call.ByName('main.Probe.Committed',text).catch(()=>{})};input.onblur=()=>mode(false);Call.ByName('main.Probe.Ready').then(()=>status.textContent='Ready for manual input').catch(()=>status.textContent='Runtime refused');</script>`

func intersects(a, b domain.Bounds) bool {
	return a.W > 0 && a.H > 0 && b.W > 0 && b.H > 0 && a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}
