package repository

// DeviceAuthCodeStatus is device_auth_codes.status.
type DeviceAuthCodeStatus string

const (
	DeviceAuthCodeStatusPending  DeviceAuthCodeStatus = "pending"
	DeviceAuthCodeStatusApproved DeviceAuthCodeStatus = "approved"
	DeviceAuthCodeStatusDenied   DeviceAuthCodeStatus = "denied"
	DeviceAuthCodeStatusExpired  DeviceAuthCodeStatus = "expired"
	// DeviceAuthCodeStatusConsumed means that the CLI received the token.
	// The stale-code sweeper does not touch a consumed code, so it does
	// not revoke a token that the CLI holds but has not used yet.
	DeviceAuthCodeStatusConsumed DeviceAuthCodeStatus = "consumed"
)

// deviceAuthCodeTransitions is the device code lifecycle. Denied, expired
// and consumed are terminal. Every UPDATE in cli_auth.go filters on a
// non-terminal source status, so SQL enforces the same rules.
var deviceAuthCodeTransitions = map[DeviceAuthCodeStatus]map[DeviceAuthCodeStatus]struct{}{
	DeviceAuthCodeStatusPending: {
		DeviceAuthCodeStatusApproved: {},
		DeviceAuthCodeStatusDenied:   {},
		DeviceAuthCodeStatusExpired:  {},
	},
	DeviceAuthCodeStatusApproved: {
		DeviceAuthCodeStatusConsumed: {},
		DeviceAuthCodeStatusExpired:  {},
	},
	DeviceAuthCodeStatusDenied:   {},
	DeviceAuthCodeStatusExpired:  {},
	DeviceAuthCodeStatusConsumed: {},
}

func (s DeviceAuthCodeStatus) Valid() bool {
	_, ok := deviceAuthCodeTransitions[s]
	return ok
}

func (s DeviceAuthCodeStatus) CanTransitionTo(next DeviceAuthCodeStatus) bool {
	nextStatuses, ok := deviceAuthCodeTransitions[s]
	if !ok {
		return false
	}
	_, ok = nextStatuses[next]
	return ok
}

func (s DeviceAuthCodeStatus) Terminal() bool {
	nextStatuses, ok := deviceAuthCodeTransitions[s]
	return ok && len(nextStatuses) == 0
}
