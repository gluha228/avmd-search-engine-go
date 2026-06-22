package supplierroutes

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	airportRoutesKey = "tf:routes:airports"
	knownAirportsKey = "tf:routes:airports:known"
	cityRoutesKey    = "tf:routes:cities"
	lastUpdateKey    = "tf:routes:last-update"
	timeLayout       = "2006-01-02T15:04:05"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) IsValidAirportRoute(ctx context.Context, originCode, destinationCode string) (bool, error) {
	return s.client.SIsMember(ctx, airportRoutesKey, originCode+destinationCode).Result()
}

func (s *RedisStore) IsKnownAirport(ctx context.Context, airportCode string) (bool, error) {
	return s.client.SIsMember(ctx, knownAirportsKey, airportCode).Result()
}

func (s *RedisStore) IsValidCityRoute(ctx context.Context, originCode, destinationCode string) (bool, error) {
	return s.client.SIsMember(ctx, cityRoutesKey, originCode+destinationCode).Result()
}

func (s *RedisStore) ReplaceRoutes(ctx context.Context, airportRoutes, cityRoutes, knownAirports []string) error {
	if err := replaceSet(ctx, s.client, airportRoutesKey, airportRoutes); err != nil {
		return err
	}
	if err := replaceSet(ctx, s.client, cityRoutesKey, cityRoutes); err != nil {
		return err
	}
	if err := replaceSet(ctx, s.client, knownAirportsKey, knownAirports); err != nil {
		return err
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
		return nil, fmt.Errorf("parse supplier routes last update: %w", err)
	}
	return &parsed, nil
}

func (s *RedisStore) SetLastUpdate(ctx context.Context, t time.Time) error {
	if err := s.client.Set(ctx, lastUpdateKey, t.Format(timeLayout), 0).Err(); err != nil {
		return fmt.Errorf("set supplier routes last update: %w", err)
	}
	return nil
}

func replaceSet(ctx context.Context, client *redis.Client, key string, values []string) error {
	pipe := client.TxPipeline()
	pipe.Del(ctx, key)
	if len(values) > 0 {
		members := make([]any, len(values))
		for i := range values {
			members[i] = values[i]
		}
		pipe.SAdd(ctx, key, members...)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("replace redis set %s: %w", key, err)
	}
	return nil
}
