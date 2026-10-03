package dock

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type (
	folderCall struct {
		ctx   context.Context
		ref   platform.FolderRef
		sort  platform.FolderSort
		reply chan platform.FolderListing
	}
	folderControllerSource struct{ calls chan folderCall }
)

func (s *folderControllerSource) ListFolder(ctx context.Context, ref platform.FolderRef, sort platform.FolderSort) (platform.FolderListing, error) {
	call := folderCall{ctx, ref, sort, make(chan platform.FolderListing, 1)}
	s.calls <- call
	result := <-call.reply
	return result, nil
}

func (s *folderControllerSource) RequestFolderAccess(context.Context, platform.FolderRef) (platform.FolderGrant, error) {
	panic("controller must not show chooser")
}

func (s *folderControllerSource) OpenFolderEntryGuarded(context.Context, platform.FolderRef, string, func() error) error {
	panic("controller must not open")
}

func folderHarness(t *testing.T) (*dockHarness, *folderControllerSource) {
	t.Helper()
	env := fake.New()
	env.ScreenList = []domain.Screen{{ID: 1, Bounds: domain.Bounds{W: 1200, H: 800}}}
	env.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "App", OnScreen: true}})
	settings := config.Default()
	settings.Dock.Enabled = false
	settings.Dock.FolderPop.Enabled = true
	settings.Dock.HoverDelayMs = 0
	obs := &testObservationSource{events: make(chan platform.DockObservation)}
	view := &dockRecordView{}
	folders := &folderControllerSource{calls: make(chan folderCall, 8)}
	c := NewController(Deps{Observations: obs, Windows: env, Env: env, View: view, Folders: folders}, settings)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	synctest.Wait()
	return &dockHarness{c: c, source: obs, view: view, env: env, settings: settings, generation: 1}, folders
}

func folderItem(path string) *platform.DockItem {
	i := dockTestItem(0)
	i.Kind = "folder"
	i.BundleID = ""
	i.Path = path
	return i
}

func folderReply(call folderCall, status string) {
	call.reply <- platform.FolderListing{FolderIdentity: call.ref.Identity, Status: status, Entries: []platform.FolderEntry{{ID: "opaque", Name: "original", Kind: "file"}}}
	synctest.Wait()
}

func TestFolderControllerLoadingNoPeriodicRelistingAndCopiedState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, s := folderHarness(t)
		h.observe(folderItem("/fixture"), 424, 750)
		loading := h.view.last()
		if loading.ContentKind != "folder" || loading.Folder == nil || loading.Folder.Status != "loading" || len(loading.Windows) != 0 {
			t.Fatal(loading)
		}
		call := <-s.calls
		folderReply(call, "ready")
		ready := h.view.last()
		if ready.Folder.Revision <= loading.Folder.Revision {
			t.Fatal("result did not advance revision")
		}
		ready.Folder.Entries[0].Name = "consumer mutation"
		h.c.SetPanelBounds(ready.Session, domain.Bounds{W: 400, H: 300})
		synctest.Wait()
		if h.view.last().Folder.Entries[0].Name != "original" {
			t.Fatal("entries alias view")
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		select {
		case <-s.calls:
			t.Fatal("periodic relist invalidated IDs")
		default:
		}
		for _, target := range h.view.targets {
			if target.Item != nil {
				t.Fatal("folder leaked into native input")
			}
		}
	})
}

func TestFolderControllerScopedRefreshAndLateResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, s := folderHarness(t)
		h.observe(folderItem("/first"), 424, 750)
		first := h.view.last()
		old := <-s.calls
		if err := h.c.SetFolderSort(first.Session, first.Folder.Revision, platform.FolderSort{Field: "size", Direction: "desc"}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if old.ctx.Err() == nil {
			t.Fatal("sort did not cancel old listing")
		}
		folderReply(old, "ready")
		next := <-s.calls
		if next.sort.Field != "size" {
			t.Fatal(next.sort)
		}
		folderReply(next, "partial")
		current := h.view.last()
		if !current.Folder.Partial {
			t.Fatal("partial lost")
		}
		h.c.RefreshFolder(first.Session, first.Folder.Revision)
		synctest.Wait()
		select {
		case <-s.calls:
			t.Fatal("stale revision caused I/O")
		default:
		}
		h.c.RefreshFolder(current.Session, current.Folder.Revision)
		synctest.Wait()
		pending := <-s.calls
		h.observe(folderItem("/second"), 424, 750)
		if pending.ctx.Err() == nil {
			t.Fatal("hover replacement did not cancel")
		}
		folderReply(pending, "revoked")
		replacement := <-s.calls
		folderReply(replacement, "missing")
		if h.view.last().Folder.FolderIdentity != "file:///second" || h.view.last().Folder.Status != "missing" {
			t.Fatal(h.view.last())
		}
		if err := h.c.SetFolderSort(current.Session, current.Folder.Revision, platform.FolderSort{Field: "shell"}); err == nil {
			t.Fatal("invalid sort accepted")
		}
	})
}

func TestFolderControllerDisableAndFolderToApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h, s := folderHarness(t)
		h.settings.Dock.Enabled = true
		h.c.Configure(h.settings)
		synctest.Wait()
		h.observe(folderItem("/fixture"), 424, 750)
		pending := <-s.calls
		h.observe(dockTestItem(10), 424, 750)
		folderReply(pending, "ready")
		synctest.Wait()
		if h.view.last().ContentKind != "windows" || h.view.last().Folder != nil {
			t.Fatal("folder result replaced app", h.view.last())
		}
		h.observe(folderItem("/fixture"), 424, 750)
		pending = <-s.calls
		h.c.Suspend(true)
		synctest.Wait()
		if !errors.Is(pending.ctx.Err(), context.Canceled) {
			t.Fatal("suspend did not cancel")
		}
		folderReply(pending, "ready")
		if h.view.hideCount() == 0 {
			t.Fatal("suspend did not hide")
		}
	})
}

func TestFolderControllerSourceStatusAndUntrustedResultBounds(t *testing.T) {
	for _, status := range []string{"permissionRequired", "revoked", "missing", "unavailable"} {
		t.Run(status, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h, s := folderHarness(t)
				h.observe(folderItem("/fixture"), 424, 750)
				call := <-s.calls
				folderReply(call, status)
				state := h.view.last()
				if state.Folder.Status != status || len(state.Folder.Entries) != 0 {
					t.Fatal(state.Folder)
				}
			})
		})
	}
	synctest.Test(t, func(t *testing.T) {
		h, s := folderHarness(t)
		h.observe(folderItem("/fixture"), 424, 750)
		call := <-s.calls
		call.reply <- platform.FolderListing{Status: "ready", Entries: []platform.FolderEntry{{ID: "invalid", Name: "missing identity"}}}
		synctest.Wait()
		if state := h.view.last(); state.Folder.Status != "unavailable" || len(state.Folder.Entries) != 0 {
			t.Fatal("unscoped result admitted", state.Folder)
		}
		state := h.view.last()
		h.c.RefreshFolder(state.Session, state.Folder.Revision)
		synctest.Wait()
		call = <-s.calls
		entries := make([]platform.FolderEntry, 502)
		call.reply <- platform.FolderListing{FolderIdentity: call.ref.Identity, Status: "ready", Entries: entries}
		synctest.Wait()
		if state = h.view.last(); len(state.Folder.Entries) != 500 || !state.Folder.Partial {
			t.Fatal("source exceeded bound", state.Folder)
		}
	})
}
