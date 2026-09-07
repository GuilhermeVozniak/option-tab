package dock

import (
	"context"
	"errors"
	"slices"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

type contentRequest struct {
	ctx                context.Context
	session, admission uint64
	kind               string
	reply              chan error
}

// SelectContent acknowledges admission on the controller loop, without waiting
// for an inventory query. A timed-out queued request cannot change a later owner.
func (c *Controller) SelectContent(session uint64, kind string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r := contentRequest{ctx: ctx, session: session, admission: c.AdmissionEpoch(), kind: kind, reply: make(chan error, 1)}
	select {
	case c.contentRequests <- r:
	case <-c.done:
		return errors.New("dock: controller stopped")
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-r.reply:
		return err
	case <-c.done:
		return errors.New("dock: controller stopped")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func contentOptions(item Item, settings config.Settings) []string {
	if item.Kind == "folder" {
		return []string{"folder"}
	}
	options := []string{}
	if settings.Dock.Enabled {
		options = append(options, "windows")
	}
	if MediaProviderForItem(item, settings.Dock.Media) != "" {
		options = append(options, "media")
	}
	return options
}

func (l *controllerLoop) selectedContent() string {
	if l.contentKind != "" {
		return l.contentKind
	}
	if l.candidate != nil && MediaProviderForItem(*l.candidate, l.settings.Dock.Media) != "" {
		return "media"
	}
	return "windows"
}

func (l *controllerLoop) selectContent(r contentRequest) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if !l.enabled() || !l.shown || l.candidate == nil || r.session == 0 || r.session != l.state.Session ||
		l.admission != r.admission || r.admission != l.controller.AdmissionEpoch() || !l.candidate.same(&l.state.Item) {
		return errors.New("dock: preview session is no longer active")
	}
	options := contentOptions(*l.candidate, l.settings)
	if (r.kind != "windows" && r.kind != "media") || !slices.Contains(options, r.kind) {
		return errors.New("dock: content is unavailable")
	}
	if r.kind == l.state.ContentKind {
		return nil
	}
	if view := l.controller.deps.View; view != nil {
		view.Hide(l.state.Session)
	}
	l.session++
	l.contentKind = r.kind
	l.measured = domain.Bounds{}
	l.state = State{
		ContentKind: r.kind, ContentOptions: options, AdmissionEpoch: l.admission, Session: l.session,
		Item: *l.candidate, Appearance: l.settings.Dock.Appearance, CardSpacingPx: l.settings.Dock.CardSpacingPx,
	}
	if r.kind == "windows" {
		l.state.EmptyReason = "loading"
	}
	l.place()
	// Publish the new owner immediately so a slow window query cannot trap the
	// user in a hidden panel. The one existing worker serializes refreshes; its
	// old session's result is discarded before the selected content is queried.
	l.publish(true)
	l.request()
	return nil
}
