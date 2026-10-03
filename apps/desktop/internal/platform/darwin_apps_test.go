//go:build darwin

package platform

import (
	"errors"
	"reflect"
	"testing"

	"option-tab/internal/domain"
)

func TestApplicationInventoryFiltersNativeRecordsWithoutRequiringWindows(t *testing.T) {
	raws := []rawApplication{
		{PID: 101, Name: "Windowless", BundleID: "test.windowless", Policy: applicationPolicyRegular, WindowCount: 0},
		{PID: 102, Name: "Visible", BundleID: "test.visible", Policy: applicationPolicyRegular, WindowCount: 2, Hidden: true},
		{PID: 103, Name: "Accessory", BundleID: "test.accessory", Policy: applicationPolicyAccessory},
		{PID: 104, Name: "Prohibited", BundleID: "test.prohibited", Policy: applicationPolicyProhibited},
		{PID: 105, Name: "Terminated", BundleID: "test.terminated", Policy: applicationPolicyRegular, Terminated: true},
		{PID: 106, Name: "Unbundled", Policy: applicationPolicyRegular},
		{PID: 107, Name: "Self", BundleID: "test.self", Policy: applicationPolicyRegular},
	}

	got := mapRawApplications(raws, 107)
	want := []domain.App{
		{ID: 101, Name: "Windowless", BundleID: "test.windowless"},
		{ID: 102, Name: "Visible", BundleID: "test.visible", Hidden: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory = %#v, want %#v", got, want)
	}
}

func TestApplicationActivationValidatesExactIdentityAndPropagatesRefusal(t *testing.T) {
	tests := []struct {
		name      string
		id        domain.AppID
		self      int
		status    applicationActivationStatus
		wantCall  bool
		wantError bool
	}{
		{name: "invalid", id: 0, self: 50, status: applicationActivationAccepted, wantError: true},
		{name: "overflow", id: domain.AppID(1 << 40), self: 50, status: applicationActivationAccepted, wantError: true},
		{name: "self", id: 50, self: 50, status: applicationActivationAccepted, wantError: true},
		{name: "accepted", id: 51, self: 50, status: applicationActivationAccepted, wantCall: true},
		{name: "vanished", id: 51, self: 50, status: applicationActivationInvalid, wantCall: true, wantError: true},
		{name: "refused", id: 51, self: 50, status: applicationActivationRefused, wantCall: true, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			err := activateApplication(tt.id, tt.self, func(pid int) applicationActivationStatus {
				called = true
				if pid != int(tt.id) {
					t.Fatalf("pid = %d, want %d", pid, tt.id)
				}
				return tt.status
			})
			if called != tt.wantCall {
				t.Fatalf("called = %v, want %v", called, tt.wantCall)
			}
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, wantError %v", err, tt.wantError)
			}
			if tt.status == applicationActivationRefused && !errors.Is(err, errApplicationActivationRefused) {
				t.Fatalf("error = %v, want refusal", err)
			}
		})
	}
}
