package proctrack

import "errors"

// ErrTrackingUnsupported indicates the macOS kqueue tracker cannot be used on this host.
var ErrTrackingUnsupported = errors.New("proctrack: tracking unsupported")

// IsTrackingUnsupported reports whether the provided error results from an unsupported tracking capability.
func IsTrackingUnsupported(err error) bool {
	return errors.Is(err, ErrTrackingUnsupported)
}
