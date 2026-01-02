package retry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestDoSucceedsFirstAttempt(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{}, func(int) error {
		attempts++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestDoRetriesThenSucceedsWithBackoff(t *testing.T) {
	attempts := 0
	var sleeps []time.Duration
	err := Do(nil, Config{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		Sleeper: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
	}, func(int) error {
		attempts++
		if attempts < 3 {
			return &RetryableError{Err: errors.New("temporary")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	wantSleeps := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
	}
	if len(sleeps) != len(wantSleeps) {
		t.Fatalf("expected %d sleeps, got %d", len(wantSleeps), len(sleeps))
	}
	for i, got := range sleeps {
		if got != wantSleeps[i] {
			t.Fatalf("sleep %d: expected %v, got %v", i, wantSleeps[i], got)
		}
	}
}

func TestDoHonorsShouldRetry(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), Config{
		MaxRetries: 3,
		ShouldRetry: func(err error) bool {
			return errors.Is(err, errStop)
		},
	}, func(int) error {
		attempts++
		return errors.New("non-retryable")
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected no retries, got %d attempts", attempts)
	}
}

var errStop = errors.New("stop")

func TestDoUsesCustomDelayFunc(t *testing.T) {
	attempts := 0
	var sleeps []time.Duration
	err := Do(nil, Config{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		DelayFunc: func(attempt int, _ error) time.Duration {
			return time.Duration(attempt+1) * time.Second
		},
		Sleeper: func(d time.Duration) {
			sleeps = append(sleeps, d)
		},
	}, func(int) error {
		attempts++
		if attempts < 3 {
			return &RetryableError{Err: errors.New("again")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	wantSleeps := []time.Duration{1 * time.Second, 2 * time.Second}
	if len(sleeps) != len(wantSleeps) {
		t.Fatalf("expected %d sleeps, got %d", len(wantSleeps), len(sleeps))
	}
	for i, got := range sleeps {
		if got != wantSleeps[i] {
			t.Fatalf("sleep %d: expected %v, got %v", i, wantSleeps[i], got)
		}
	}
}

func TestDoStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := Do(ctx, Config{
		MaxRetries: 5,
		BaseDelay:  1 * time.Millisecond,
	}, func(int) error {
		attempts++
		cancel()
		return errors.New("fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected stop after first attempt due to cancel, got %d", attempts)
	}
}

func TestRetryAfterDelayParsesSeconds(t *testing.T) {
	fb := 5 * time.Second
	got := RetryAfterDelay("10", fb)
	if got != 10*time.Second {
		t.Fatalf("expected 10s, got %v", got)
	}
}

func TestRetryAfterDelayParsesHTTPDate(t *testing.T) {
	now := time.Now()
	header := now.Add(3 * time.Second).UTC().Format(http.TimeFormat)
	fb := 1 * time.Second
	got := RetryAfterDelay(header, fb)
	if got < 2*time.Second || got > 4*time.Second {
		t.Fatalf("expected about 3s, got %v", got)
	}
}

func TestRetryAfterDelayFallbackOnInvalid(t *testing.T) {
	fb := 2 * time.Second
	got := RetryAfterDelay("not-a-date", fb)
	if got != fb {
		t.Fatalf("expected fallback %v, got %v", fb, got)
	}
}

func TestIsRetryable(t *testing.T) {
	base := errors.New("base")
	r := &RetryableError{Err: base}
	if !IsRetryable(r) {
		t.Fatalf("expected retryable")
	}
	wrapped := fmt.Errorf("wrap: %w", r)
	if !IsRetryable(wrapped) {
		t.Fatalf("expected wrapped retryable")
	}
	if IsRetryable(base) {
		t.Fatalf("expected non-retryable base")
	}
}

func TestDoReturnsLastErrorWhenExhausted(t *testing.T) {
	lastErr := errors.New("last")
	err := Do(context.Background(), Config{
		MaxRetries: 2,
		BaseDelay:  1 * time.Millisecond,
	}, func(int) error {
		return lastErr
	})
	if !errors.Is(err, lastErr) {
		t.Fatalf("expected lastErr, got %v", err)
	}
}
