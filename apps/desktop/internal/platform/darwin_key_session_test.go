//go:build darwin

package platform

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeKeyQueuePreservesOriginatingSession(t *testing.T) {
	h := newDarwinHotkeys()
	defer h.keys.close()
	h.SetKeySession(41)
	h.enqueueKey(keyEventFromTap(0, 0, "a"))
	h.SetKeySession(42)
	h.enqueueKey(keyEventFromTap(1, 0, "s"))
	h.SetKeySession(0)
	old, ok := h.keys.pop()
	if !ok || old.Session != 41 || old.Key != "a" {
		t.Fatalf("queued old key was retagged: %+v", old)
	}
	current, ok := h.keys.pop()
	if !ok || current.Session != 42 || current.Key != "s" {
		t.Fatalf("queued current key was retagged: %+v", current)
	}
	h.enqueueKey(keyEventFromTap(0, 0, "a"))
	unowned, _ := h.keys.pop()
	if unowned.Session != 0 {
		t.Fatalf("closed session still tagged: %+v", unowned)
	}
}

func TestNativeKeySessionJSONIsOptional(t *testing.T) {
	zero, err := json.Marshal(KeyEvent{Key: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(zero), "session") {
		t.Fatalf("zero token must preserve optional compatibility: %s", zero)
	}
	tagged, err := json.Marshal(KeyEvent{Session: 41, Key: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tagged), `"session":41`) {
		t.Fatalf("missing native token: %s", tagged)
	}
}
