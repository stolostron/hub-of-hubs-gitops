package helpers

import (
	"time"
)

const (
	// RequeuePeriod is the time to wait until reconciliation retry in failure cases.
	RequeuePeriod = 5 * time.Second
)
