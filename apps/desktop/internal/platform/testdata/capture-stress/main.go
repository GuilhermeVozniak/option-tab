package main

/*
#include <mach/mach.h>
static unsigned long long probe_current_rss(void) {
  struct mach_task_basic_info info;
  mach_msg_type_number_t count = MACH_TASK_BASIC_INFO_COUNT;
  kern_return_t result = task_info(mach_task_self(), MACH_TASK_BASIC_INFO, (task_info_t)&info, &count);
  return result == KERN_SUCCESS ? info.resident_size : 0;
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/preview"
)

type identitySource interface {
	WindowIdentity(domain.WindowID) (platform.AutomationWindowIdentity, error)
	WindowIdentityCurrent(platform.AutomationWindowIdentity) bool
}
type trackedSource struct {
	platform.IdentityWindowCaptureSource
	identitySource
	active      atomic.Int64
	peak        atomic.Int64
	starts      atomic.Int64
	completed   atomic.Int64
	frames      atomic.Int64
	mu          sync.Mutex
	errors      []string
	frameCounts map[domain.WindowID]int
}

func (s *trackedSource) StreamWindow(c context.Context, id domain.WindowID, px int, f func(string)) error {
	panic("identity binding required")
}

func (s *trackedSource) ThumbnailDataURL(id domain.WindowID, px int) string {
	panic("identity binding required")
}

func (s *trackedSource) StreamWindowWithIdentity(c context.Context, id platform.AutomationWindowIdentity, px int, f func(string)) error {
	n := s.active.Add(1)
	s.starts.Add(1)
	for old := s.peak.Load(); n > old && !s.peak.CompareAndSwap(old, n); old = s.peak.Load() {
	}
	defer s.active.Add(-1)
	defer s.completed.Add(1)
	err := s.IdentityWindowCaptureSource.StreamWindowWithIdentity(c, id, px, func(url string) { s.frames.Add(1); s.mu.Lock(); s.frameCounts[id.ID]++; s.mu.Unlock(); f(url) })
	if err != nil && c.Err() == nil {
		s.mu.Lock()
		s.errors = append(s.errors, err.Error())
		s.mu.Unlock()
	}
	return err
}

func usage() map[string]any {
	var r syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// maxrss is bytes on Darwin. Current RSS is collected independently by the driver.
	return map[string]any{"rssBytes": uint64(C.probe_current_rss()), "maxRssBytes": r.Maxrss, "cpuSeconds": float64(r.Utime.Sec+r.Stime.Sec) + float64(r.Utime.Usec+r.Stime.Usec)/1e6, "heapAllocBytes": m.HeapAlloc, "goroutines": runtime.NumGoroutine()}
}

func emit(stage string, s *trackedSource, extra map[string]any) {
	record := usage()
	record["stage"] = stage
	record["time"] = time.Now().UTC()
	record["active"] = s.active.Load()
	record["peak"] = s.peak.Load()
	record["starts"] = s.starts.Load()
	record["completed"] = s.completed.Load()
	record["frames"] = s.frames.Load()
	for k, v := range extra {
		record[k] = v
	}
	_ = json.NewEncoder(os.Stdout).Encode(record)
}

func waitUntil(timeout time.Duration, predicate func() bool) bool {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if predicate() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return predicate()
}

func main() {
	p, err := platform.New()
	if err != nil {
		panic(err)
	}
	s := &trackedSource{IdentityWindowCaptureSource: p.(platform.IdentityWindowCaptureSource), identitySource: p.(identitySource), frameCounts: map[domain.WindowID]int{}}
	var ids []domain.WindowID
	bound := map[domain.WindowID]platform.AutomationWindowIdentity{}
	for _, raw := range strings.Split(os.Getenv("CAPTURE_WINDOW_IDS"), ",") {
		n, e := strconv.ParseUint(raw, 10, 64)
		if e != nil {
			panic(e)
		}
		id := domain.WindowID(n)
		identity, e := s.WindowIdentity(id)
		if e != nil {
			panic(e)
		}
		if int(identity.Process.PID) != mustPID() {
			panic("nonfixture window refused")
		}
		ids = append(ids, id)
		bound[id] = identity
	}
	if len(ids) != 4 {
		panic("four disposable windows required")
	}
	received := atomic.Int64{}
	a := preview.New(s, func(_ domain.WindowID, url string) {
		if url != "" {
			received.Add(1)
		}
	})
	b := a.NewPeer(func(_ domain.WindowID, url string) {
		if url != "" {
			received.Add(1)
		}
	})
	defer func() { <-a.CloseAndDrain(); <-b.CloseAndDrain() }()
	showA := func() { a.UpdateBound(ids[:2], ids[0], 280, bound) }
	showB := func() { b.UpdateBound(ids[2:], ids[2], 280, bound) }
	hide := func() { a.UpdateBound(nil, 0, 280, nil); b.UpdateBound(nil, 0, 280, nil) }
	emit("baseline", s, nil)
	time.Sleep(3 * time.Second)
	emit("baseline-idle", s, nil)
	failures := []string{}
	for cycle := 0; cycle < 93; cycle++ {
		s.mu.Lock()
		before := map[domain.WindowID]int{}
		for _, id := range ids {
			before[id] = s.frameCounts[id]
		}
		s.mu.Unlock()
		showA()
		showB()
		if !waitUntil(5*time.Second, func() bool {
			if s.active.Load() != 4 {
				return false
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, id := range ids {
				if s.frameCounts[id] < before[id]+2 {
					return false
				}
			}
			return true
		}) {
			failures = append(failures, fmt.Sprintf("cycle %d did not receive four live streams", cycle))
			break
		}
		time.Sleep(450 * time.Millisecond)
		// Retire alternating owners while the peer remains active, then hide both.
		if cycle%2 == 0 {
			a.UpdateBound(nil, 0, 280, nil)
		} else {
			b.UpdateBound(nil, 0, 280, nil)
		}
		if !waitUntil(3*time.Second, func() bool { return s.active.Load() == 2 }) {
			failures = append(failures, fmt.Sprintf("cycle %d first owner did not drain", cycle))
			break
		}
		hide()
		if !waitUntil(3*time.Second, func() bool { return s.active.Load() == 0 && s.starts.Load() == s.completed.Load() }) {
			failures = append(failures, fmt.Sprintf("cycle %d native sources did not drain", cycle))
			break
		}
		stopped := s.frames.Load()
		delivered := received.Load()
		time.Sleep(200 * time.Millisecond)
		if stopped != s.frames.Load() || delivered != received.Load() {
			failures = append(failures, fmt.Sprintf("cycle %d delivered after drain", cycle))
		}
		if len(a.CachedFrames()) != 0 || len(b.CachedFrames()) != 0 {
			failures = append(failures, fmt.Sprintf("cycle %d cache survived hide", cycle))
		}
		emit("cycle", s, map[string]any{"cycle": cycle, "warmup": cycle < 3})
		if cycle == 32 || cycle == 62 || cycle == 92 {
			time.Sleep(5 * time.Second)
			emit("batch-settled", s, map[string]any{"cycle": cycle})
		}
	}
	hide()
	<-a.CloseAndDrain()
	<-b.CloseAndDrain()
	emit("closed", s, nil)
	time.Sleep(5 * time.Second)
	emit("settled", s, nil)
	if s.peak.Load() > 4 {
		failures = append(failures, "four-stream budget exceeded")
	}
	if s.starts.Load() != 372 || s.completed.Load() != 372 {
		failures = append(failures, "93 complete four-stream cycles required")
	}
	s.mu.Lock()
	nativeErrors := append([]string(nil), s.errors...)
	s.mu.Unlock()
	if len(nativeErrors) > 0 {
		failures = append(failures, "native stream error")
	}
	emit("result", s, map[string]any{"failures": failures, "nativeErrors": nativeErrors, "delivered": received.Load(), "passed": len(failures) == 0})
	if len(failures) > 0 {
		os.Exit(1)
	}
}

func mustPID() int {
	n, e := strconv.Atoi(os.Getenv("CAPTURE_FIXTURE_PID"))
	if e != nil {
		panic(e)
	}
	return n
}
