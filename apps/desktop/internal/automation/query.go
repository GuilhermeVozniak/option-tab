package automation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"strconv"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func processView(p platform.ProcessIdentity) ProcessView {
	return ProcessView{p.PID, strconv.FormatUint(p.StartSeconds, 10), p.StartMicros}
}

func windowView(w domain.Window, id platform.AutomationWindowIdentity) WindowView {
	return WindowView{WindowID: strconv.FormatUint(uint64(w.ID), 10), AppID: w.AppID, AppName: w.AppName, BundleID: w.BundleID, Title: w.Title, Bounds: w.Bounds, ScreenID: w.ScreenID, SpaceID: strconv.FormatUint(uint64(w.SpaceID), 10), Minimized: w.Minimized, Hidden: w.Hidden, Fullscreen: w.Fullscreen, OnScreen: w.OnScreen, Process: processView(id.Process)}
}

func (s *Service) queryApps(ctx context.Context, out *envelope) error {
	apps, err := s.apps(ctx)
	if err != nil {
		return err
	}
	views := make([]AppView, 0, min(len(apps), MaxEntries))
	out.Apps = &views
	budget := MaxReplyBytes - 2048
	for _, a := range apps {
		if len(views) == MaxEntries || !validText(a.Name, 64*1024) || !validText(a.BundleID, 64*1024) {
			out.Omitted++
			continue
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		p, e := s.process(a.ID)
		if e != nil {
			return e
		}
		view := AppView{a.ID, a.Name, a.BundleID, a.Hidden, processView(p)}
		encoded, e := json.Marshal(view)
		if e != nil {
			return e
		}
		if len(encoded)+1 > budget {
			out.Omitted++
			continue
		}
		budget -= len(encoded) + 1
		views = append(views, view)
	}
	out.Truncated = out.Omitted > 0 || out.ImagesOmitted > 0
	return nil
}

func (s *Service) queryWindows(ctx context.Context, r platform.AutomationRequest, out *envelope) error {
	var selected *platform.ProcessIdentity
	if r.App != nil {
		_, p, e := s.resolveApp(ctx, *r.App)
		if e != nil {
			return e
		}
		selected = &p
	}
	windows, err := s.windows(ctx)
	if err != nil {
		return err
	}
	views := make([]WindowView, 0, min(len(windows), MaxEntries))
	ids := make([]platform.AutomationWindowIdentity, 0, cap(views))
	out.Windows = &views
	budget := MaxReplyBytes - 2048
	for _, w := range windows {
		if selected != nil && w.AppID != selected.PID {
			continue
		}
		if len(views) == MaxEntries || !validText(w.Title, 64*1024) || !validText(w.AppName, 64*1024) || !validText(w.BundleID, 64*1024) {
			out.Omitted++
			continue
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		id, e := s.windowIdentity(w)
		if e != nil {
			return e
		}
		if selected != nil && id.Process != *selected {
			return failure("staleIdentity", "selected application process changed")
		}
		view := windowView(w, id)
		encoded, e := json.Marshal(view)
		if e != nil {
			return e
		}
		if len(encoded)+1 > budget {
			out.Omitted++
			continue
		}
		budget -= len(encoded) + 1
		views = append(views, view)
		ids = append(ids, id)
	}
	if selected != nil {
		if err = s.processGuard(ctx, r.Operation, *selected)(); err != nil {
			return err
		}
	}
	if r.IncludeImages {
		pointers := make([]*WindowView, len(views))
		for i := range views {
			pointers[i] = &views[i]
		}
		s.images(ctx, r.Operation, pointers, ids, out)
	}
	out.Truncated = out.Omitted > 0 || out.ImagesOmitted > 0
	return nil
}

func (s *Service) images(ctx context.Context, op platform.AutomationOperation, views []*WindowView, ids []platform.AutomationWindowIdentity, out *envelope) {
	for _, v := range views {
		v.ImageStatus = "missing"
	}
	if s.deps.CachedFrames == nil {
		for _, v := range views {
			v.ImageStatus = "unavailable"
		}
		return
	}
	frames, err := s.deps.CachedFrames(ctx)
	if err != nil {
		for _, v := range views {
			v.ImageStatus = "unavailable"
		}
		return
	}
	now := s.deps.Now()
	count := 0
	for i, v := range views {
		if ctx.Err() != nil {
			return
		}
		var frame *CachedFrame
		matches := 0
		for j := range frames {
			if frames[j].Window.ID == ids[i].ID {
				matches++
				frame = &frames[j]
			}
		}
		if matches == 0 {
			continue
		}
		if matches != 1 || frame.Window != ids[i] {
			v.ImageStatus = "stale"
			continue
		}
		age := now.Sub(frame.CapturedAt)
		if frame.CapturedAt.IsZero() || age < 0 || age > MaxImageAge {
			v.ImageStatus = "stale"
			continue
		}
		if count == MaxImages || base64.StdEncoding.EncodedLen(len(frame.PNG))+len("data:image/png;base64,") > MaxImageBytes {
			v.ImageStatus = "omitted"
			out.ImagesOmitted++
			continue
		}
		cfg, e := png.DecodeConfig(bytes.NewReader(frame.PNG))
		if e != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 16_777_216 {
			v.ImageStatus = "unavailable"
			continue
		}
		if s.windowGuard(ctx, op, ids[i])() != nil {
			v.ImageStatus = "stale"
			continue
		}
		data := append([]byte(nil), frame.PNG...)
		v.Image = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
		captured := frame.CapturedAt
		v.CapturedAt = &captured
		v.ImageStatus = "cached"
		count++
	}
}

func encodeBounded(out *envelope) ([]byte, error) {
	for {
		data, err := json.Marshal(out)
		if err != nil {
			return nil, failure("unavailable", "inventory contains invalid JSON values")
		}
		if len(data) <= MaxReplyBytes {
			return data, nil
		}
		removed := false
		if out.Windows != nil {
			for i := len(*out.Windows) - 1; i >= 0; i-- {
				v := &(*out.Windows)[i]
				if v.Image != "" {
					v.Image = ""
					v.CapturedAt = nil
					v.ImageStatus = "omitted"
					out.ImagesOmitted++
					removed = true
					break
				}
			}
		}
		if !removed && out.Active != nil && out.Active.Image != "" {
			out.Active.Image = ""
			out.Active.CapturedAt = nil
			out.Active.ImageStatus = "omitted"
			out.ImagesOmitted++
			removed = true
		}
		if removed {
			out.Truncated = true
			continue
		}
		if out.Windows != nil && len(*out.Windows) > 0 {
			*out.Windows = (*out.Windows)[:len(*out.Windows)-1]
		} else if out.Apps != nil && len(*out.Apps) > 0 {
			*out.Apps = (*out.Apps)[:len(*out.Apps)-1]
		} else {
			return nil, failure("unavailable", "automation reply exceeds byte limit")
		}
		out.Omitted++
		out.Truncated = true
	}
}
