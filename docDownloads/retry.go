package docdownload

import "time"

// Do runs fn up to attempts times with exponential backoff starting at baseDelay.
// It returns the last error (or nil if success).
func Do(attempts int, baseDelay time.Duration, fn func() error) error {
	if attempts <= 0 {
		return fn()
	}
	delay := baseDelay
	var lastErr error
	for i := 0; i < attempts; i++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return lastErr
}
