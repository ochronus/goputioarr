package retry

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"
)

// Config controls retry behavior.
type Config struct {
	// MaxRetries is the total number of attempts (including the first).
	// If zero or negative, DefaultMaxRetries is used.
	MaxRetries int

	// BaseDelay is the starting delay. Each retry is doubled (exponential backoff).
	// If zero, DefaultBaseDelay is used.
	BaseDelay time.Duration

	// DelayFunc, if provided, overrides the backoff calculation per attempt and error.
	// Return a negative duration to skip sleeping for that attempt.
	DelayFunc func(attempt int, err error) time.Duration

	// ShouldRetry, if provided, determines whether to retry based on the returned error.
	// If nil, any non-nil error is retried (until attempts are exhausted).
	ShouldRetry func(error) bool

	// Sleeper allows tests to override sleeping. If nil, time.Sleep is used when
	// ctx is nil. When ctx is non-nil, a timer/select is used to honor cancellation.
	Sleeper func(time.Duration)
}

const (
	DefaultMaxRetries = 3
	DefaultBaseDelay  = 200 * time.Millisecond
)

// RetryableError marks an error as explicitly retryable.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	if e.Err == nil {
		return "retryable error"
	}
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable reports whether err or any wrapped error is a RetryableError.
func IsRetryable(err error) bool {
	var r *RetryableError
	return errors.As(err, &r)
}

// RetryAfterDelay parses an HTTP Retry-After header value and returns the advised
// delay. If parsing fails or the header is empty, fallback is returned.
func RetryAfterDelay(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}

	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}

	if ts, err := http.ParseTime(header); err == nil {
		now := time.Now()
		if ts.After(now) {
			return ts.Sub(now)
		}
		return 0
	}

	return fallback
}

// Do executes op with exponential backoff. It retries up to MaxRetries times,
// sleeping BaseDelay * 2^attempt (or DelayFunc) between retries. If ctx is
// canceled, the context error is returned immediately.
func Do(ctx context.Context, cfg Config, op func(attempt int) error) error {
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	base := cfg.BaseDelay
	if base <= 0 {
		base = DefaultBaseDelay
	}

	shouldRetry := cfg.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = func(err error) bool { return err != nil }
	}

	sleeper := cfg.Sleeper
	if sleeper == nil {
		sleeper = time.Sleep
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Honor cancellation before each attempt.
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		err := op(attempt)
		if err == nil {
			return nil
		}
		lastErr = err

		// Stop if we shouldn't retry this error or we're out of attempts.
		if attempt == maxRetries-1 || !shouldRetry(err) {
			return lastErr
		}

		// Sleep with exponential backoff. No jitter for determinism in tests.
		delay := base * time.Duration(1<<attempt)
		if cfg.DelayFunc != nil {
			delay = cfg.DelayFunc(attempt, err)
		}

		if delay < 0 {
			continue
		}

		if ctx != nil {
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		} else {
			sleeper(delay)
		}
	}

	return lastErr
}
