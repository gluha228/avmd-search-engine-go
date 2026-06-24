package redishealth

import (
	"avmd-search-engine-go/internal/config"
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitUsesDefaultThreeAttempts(t *testing.T) {
	calls := 0
	errUnavailable := errors.New("redis unavailable")

	err := wait(context.Background(), func(context.Context) error {
		calls++
		return errUnavailable
	}, WaitOptions{RetryDelay: 0}, nil)

	if err == nil {
		t.Fatal("expected wait to return an error")
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts by default, got %d", calls)
	}
}

func TestWaitStopsAfterSuccessfulAttempt(t *testing.T) {
	calls := 0

	err := wait(context.Background(), func(context.Context) error {
		calls++
		if calls == 2 {
			return nil
		}
		return errors.New("redis unavailable")
	}, WaitOptions{MaxAttempts: 3, RetryDelay: 0}, nil)

	if err != nil {
		t.Fatalf("expected wait to succeed, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls)
	}
}

func TestWaitOptionsFromConfigUsesEnvDefaults(t *testing.T) {
	opts := WaitOptionsFromConfig(&config.Config{})

	if opts.MaxAttempts != 3 {
		t.Fatalf("expected default 3 attempts, got %d", opts.MaxAttempts)
	}
	if opts.RetryDelay != time.Second {
		t.Fatalf("expected default 1s delay, got %v", opts.RetryDelay)
	}
}

func TestWaitOptionsFromConfigUsesConfiguredValues(t *testing.T) {
	opts := WaitOptionsFromConfig(&config.Config{
		RedisConnectAttempts:          5,
		RedisConnectRetryDelaySeconds: 2,
	})

	if opts.MaxAttempts != 5 {
		t.Fatalf("expected 5 attempts, got %d", opts.MaxAttempts)
	}
	if opts.RetryDelay != 2*time.Second {
		t.Fatalf("expected 2s delay, got %v", opts.RetryDelay)
	}
}
