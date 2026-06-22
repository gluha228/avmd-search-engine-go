package calendar

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const priceKeyPrefix = "flight:price:"

type RedisPriceStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisPriceStore(client *redis.Client, ttl time.Duration) *RedisPriceStore {
	return &RedisPriceStore{
		client: client,
		ttl:    ttl,
	}
}

func (s *RedisPriceStore) GetMinPrice(ctx context.Context, origin, destination string, date time.Time) (*PriceEntry, error) {
	value, err := s.client.Get(ctx, buildPriceKey(origin, destination, date)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get calendar price: %w", err)
	}
	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("parse calendar price: %w", err)
	}
	return &PriceEntry{Price: price}, nil
}

func (s *RedisPriceStore) SetMinPriceIfLower(ctx context.Context, origin, destination string, date time.Time, entry PriceEntry) error {
	if entry.Price <= 0 {
		return nil
	}
	cached, err := s.GetMinPrice(ctx, origin, destination, date)
	if err != nil {
		return err
	}
	if cached != nil && cached.Price > 0 && cached.Price <= entry.Price {
		return nil
	}
	return s.SetMinPrice(ctx, origin, destination, date, entry)
}

func (s *RedisPriceStore) SetMinPrice(ctx context.Context, origin, destination string, date time.Time, entry PriceEntry) error {
	value := strconv.FormatFloat(entry.Price, 'f', -1, 64)
	if err := s.client.Set(ctx, buildPriceKey(origin, destination, date), value, s.ttl).Err(); err != nil {
		return fmt.Errorf("set calendar price: %w", err)
	}
	return nil
}

func buildPriceKey(origin, destination string, date time.Time) string {
	return priceKeyPrefix +
		strings.ToUpper(origin) + ":" +
		strings.ToUpper(destination) + ":" +
		date.Format(time.DateOnly)
}
