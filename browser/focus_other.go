//go:build !darwin

package browser

import "time"

// KeepFront runs fn. Handing the focus back is only implemented on macOS.
func KeepFront(_ time.Duration, fn func() error) error {
	return fn()
}
