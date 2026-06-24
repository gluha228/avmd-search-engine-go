package googlesheets

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

const (
	contactDetailsEnabledKey         = "google:sheets:contact-details:enabled"
	contactDetailsCredentialsJSONKey = "google:sheets:contact-details:credentials-json"
	contactDetailsSpreadsheetIDKey   = "google:sheets:contact-details:spreadsheet-id"
	contactDetailsRangeKey           = "google:sheets:contact-details:range"
)

type ContactDetailsConfig struct {
	Enabled         bool
	CredentialsJSON []byte
	SpreadsheetID   string
	Range           string
}

type RedisConfigStore struct {
	client *redis.Client
}

func NewRedisConfigStore(client *redis.Client) *RedisConfigStore {
	return &RedisConfigStore{client: client}
}

func (s *RedisConfigStore) ContactDetailsConfig(ctx context.Context) (ContactDetailsConfig, error) {
	enabledValue, err := s.client.Get(ctx, contactDetailsEnabledKey).Result()
	if err == redis.Nil {
		return ContactDetailsConfig{}, nil
	}
	if err != nil {
		return ContactDetailsConfig{}, fmt.Errorf("get Google Sheets contact details enabled flag: %w", err)
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(enabledValue))
	if err != nil {
		return ContactDetailsConfig{}, fmt.Errorf("parse Google Sheets contact details enabled flag: %w", err)
	}
	if !enabled {
		return ContactDetailsConfig{}, nil
	}

	values, err := s.client.MGet(
		ctx,
		contactDetailsCredentialsJSONKey,
		contactDetailsSpreadsheetIDKey,
		contactDetailsRangeKey,
	).Result()
	if err != nil {
		return ContactDetailsConfig{}, fmt.Errorf("get Google Sheets contact details config: %w", err)
	}
	cfg := ContactDetailsConfig{Enabled: true}
	if len(values) == 3 {
		cfg.CredentialsJSON = []byte(stringValue(values[0]))
		cfg.SpreadsheetID = strings.TrimSpace(stringValue(values[1]))
		cfg.Range = strings.TrimSpace(stringValue(values[2]))
	}
	if len(cfg.CredentialsJSON) == 0 {
		return ContactDetailsConfig{}, fmt.Errorf("%s is required when %s=true", contactDetailsCredentialsJSONKey, contactDetailsEnabledKey)
	}
	if cfg.SpreadsheetID == "" {
		return ContactDetailsConfig{}, fmt.Errorf("%s is required when %s=true", contactDetailsSpreadsheetIDKey, contactDetailsEnabledKey)
	}
	if cfg.Range == "" {
		return ContactDetailsConfig{}, fmt.Errorf("%s is required when %s=true", contactDetailsRangeKey, contactDetailsEnabledKey)
	}
	return cfg, nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}
