package repository

import "testing"

func TestDeviceAuthCodeStatusCanTransitionTo(t *testing.T) {
	all := []DeviceAuthCodeStatus{
		DeviceAuthCodeStatusPending,
		DeviceAuthCodeStatusApproved,
		DeviceAuthCodeStatusDenied,
		DeviceAuthCodeStatusExpired,
		DeviceAuthCodeStatusConsumed,
	}
	allowed := map[DeviceAuthCodeStatus][]DeviceAuthCodeStatus{
		DeviceAuthCodeStatusPending:  {DeviceAuthCodeStatusApproved, DeviceAuthCodeStatusDenied, DeviceAuthCodeStatusExpired},
		DeviceAuthCodeStatusApproved: {DeviceAuthCodeStatusConsumed, DeviceAuthCodeStatusExpired},
	}
	if len(all) != len(deviceAuthCodeTransitions) {
		t.Fatalf("test covers %d statuses, table has %d", len(all), len(deviceAuthCodeTransitions))
	}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, next := range allowed[from] {
				if next == to {
					want = true
				}
			}
			if got := from.CanTransitionTo(to); got != want {
				t.Errorf("%s -> %s = %v, want %v", from, to, got, want)
			}
		}
	}
	if DeviceAuthCodeStatus("revoked").Valid() {
		t.Error("unknown status must not be valid")
	}
}

func TestDeviceAuthCodeTerminalStatuses(t *testing.T) {
	terminal := map[DeviceAuthCodeStatus]bool{
		DeviceAuthCodeStatusPending:  false,
		DeviceAuthCodeStatusApproved: false,
		DeviceAuthCodeStatusDenied:   true,
		DeviceAuthCodeStatusExpired:  true,
		DeviceAuthCodeStatusConsumed: true,
	}
	for status, want := range terminal {
		if got := status.Terminal(); got != want {
			t.Errorf("%s terminal = %v, want %v", status, got, want)
		}
	}
	// A code that the CLI exchanged for a token must never expire, because
	// the sweeper revokes the tokens of expired codes.
	if DeviceAuthCodeStatusConsumed.CanTransitionTo(DeviceAuthCodeStatusExpired) {
		t.Error("consumed -> expired must be refused")
	}
}
