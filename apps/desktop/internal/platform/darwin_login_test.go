//go:build darwin

package platform

import "testing"

func TestLoginItemResult(t *testing.T) {
	if err := loginItemResult(0); err == nil {
		t.Fatal("native registration failure must be reported")
	}
	if err := loginItemResult(1); err != nil {
		t.Fatalf("native success failed: %v", err)
	}
}
