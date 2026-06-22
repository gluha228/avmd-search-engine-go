package currencies

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	rateKeyPrefix = "currencies:rate:"
	lastUpdateKey = "currencies:last-update"
	timeLayout    = "2006-01-02T15:04:05"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) GetRate(ctx context.Context, currencyCode string) (float64, error) {
	value, err := s.client.Get(ctx, rateKey(currencyCode)).Result()
	if err != nil {
		return 0, err
	}
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse currency rate: %w", err)
	}
	return rate, nil
}

func (s *RedisStore) SetRate(ctx context.Context, currencyCode string, rate float64) error {
	if err := s.client.Set(ctx, rateKey(currencyCode), strconv.FormatFloat(rate, 'f', -1, 64), 0).Err(); err != nil {
		return fmt.Errorf("set currency rate: %w", err)
	}
	return nil
}

func (s *RedisStore) LastUpdate(ctx context.Context) (*time.Time, error) {
	value, err := s.client.Get(ctx, lastUpdateKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	parsed, err := time.ParseInLocation(timeLayout, value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("parse currency last update: %w", err)
	}
	return &parsed, nil
}

func (s *RedisStore) SetLastUpdate(ctx context.Context, t time.Time) error {
	if err := s.client.Set(ctx, lastUpdateKey, t.Format(timeLayout), 0).Err(); err != nil {
		return fmt.Errorf("set currency last update: %w", err)
	}
	return nil
}

func rateKey(currencyCode string) string {
	return rateKeyPrefix + strings.ToUpper(strings.TrimSpace(currencyCode))
}
