package apitoken

import "time"

// SetNow makes the Authenticator take the time from now
func SetNow(a *Authenticator, now func() time.Time) {
	a.now = now
}
