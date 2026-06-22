package session

import (
	"avmd-search-engine-go/internal/flights"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const keyPrefix = "flight:session:"

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

func NewRedisStore(client *redis.Client, ttl time.Duration, logger *slog.Logger) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    ttl,
		logger: logger,
	}
}

func (s *RedisStore) Create(ctx context.Context, session flights.FlightSearchSession) (string, error) {
	searchID := uuid.NewString()
	if err := s.Save(ctx, searchID, session); err != nil {
		return "", err
	}
	if s.logger != nil {
		s.logger.Debug("created flight search session", "search_id", searchID, "ttl", s.ttl.String())
	}
	return searchID, nil
}

func (s *RedisStore) Save(ctx context.Context, searchID string, session flights.FlightSearchSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal flight search session: %w", err)
	}
	if err := s.client.Set(ctx, keyPrefix+searchID, payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("save flight search session to redis: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, searchID string) (*flights.FlightSearchSession, error) {
	payload, err := s.client.Get(ctx, keyPrefix+searchID).Bytes()
	if err != nil {
		return nil, fmt.Errorf("get flight search session from redis: %w", err)
	}
	var session flights.FlightSearchSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("unmarshal flight search session: %w", err)
	}
	return &session, nil
}
