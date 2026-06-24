package redishealth

import (
	"avmd-search-engine-go/internal/config"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisLogger struct {
	logger *slog.Logger
}

type WaitOptions struct {
	MaxAttempts int
	RetryDelay  time.Duration
}

func ConfigureLogger(logger *slog.Logger) {
	if logger == nil {
		return
	}
	redis.SetLogger(redisLogger{logger: logger})
}

func NewClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

func NewPreflightClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:               cfg.RedisAddr,
		Password:           cfg.RedisPassword,
		DB:                 cfg.RedisDB,
		MaxRetries:         -1,
		DialerRetries:      1,
		DialerRetryTimeout: 0,
		DialerRetryBackoff: func(int) time.Duration { return 0 },
	})
}

func WaitOptionsFromConfig(cfg *config.Config) WaitOptions {
	attempts := cfg.RedisConnectAttempts
	if attempts <= 0 {
		attempts = 3
	}
	delaySeconds := cfg.RedisConnectRetryDelaySeconds
	if delaySeconds <= 0 {
		delaySeconds = 1
	}
	return WaitOptions{
		MaxAttempts: attempts,
		RetryDelay:  time.Duration(delaySeconds) * time.Second,
	}
}

func (l redisLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	if l.logger == nil {
		return
	}
	message := strings.TrimSpace(fmt.Sprintf(format, v...))
	if message == "" {
		return
	}
	l.logger.DebugContext(ctx, "redis client internal log", "message", message)
}

func Wait(ctx context.Context, client *redis.Client, opts WaitOptions, logger *slog.Logger) error {
	return wait(ctx, func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}, opts, logger)
}

func wait(ctx context.Context, ping func(context.Context) error, opts WaitOptions, logger *slog.Logger) error {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		if err := ping(ctx); err != nil {
			lastErr = err
			if logger != nil {
				logger.Error(
					"redis is unavailable, waiting before starting dependent services",
					"attempt", attempt,
					"max_attempts", opts.MaxAttempts,
					"error", err,
				)
			}
			if attempt == opts.MaxAttempts {
				break
			}
			if opts.RetryDelay > 0 {
				timer := time.NewTimer(opts.RetryDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("wait for redis: %w", ctx.Err())
				case <-timer.C:
				}
			}
			continue
		}
		if logger != nil {
			logger.Debug("redis connection established")
		}
		return nil
	}
	return fmt.Errorf("redis unavailable after %d connection attempts: %w", opts.MaxAttempts, lastErr)
}
