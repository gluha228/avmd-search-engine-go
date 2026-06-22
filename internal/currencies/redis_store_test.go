package currencies

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
)

func TestRedisStoreStoresRateAndLastUpdate(t *testing.T) {
	client, mock := redismock.NewClientMock()
	store := NewRedisStore(client)
	ctx := context.Background()
	now := time.Date(2026, 7, 1, 3, 0, 0, 0, time.UTC)

	mock.ExpectSet("currencies:rate:EUR", "0.9", 0).SetVal("OK")
	if err := store.SetRate(ctx, "eur", 0.9); err != nil {
		t.Fatalf("SetRate returned error: %v", err)
	}

	mock.ExpectGet("currencies:rate:EUR").SetVal("0.9")
	rate, err := store.GetRate(ctx, "EUR")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}
	if rate != 0.9 {
		t.Fatalf("expected 0.9, got %v", rate)
	}

	mock.ExpectGet("currencies:last-update").RedisNil()
	lastUpdate, err := store.LastUpdate(ctx)
	if err != nil {
		t.Fatalf("LastUpdate returned error: %v", err)
	}
	if lastUpdate != nil {
		t.Fatalf("expected nil last update, got %v", lastUpdate)
	}

	mock.ExpectSet("currencies:last-update", "2026-07-01T03:00:00", 0).SetVal("OK")
	if err := store.SetLastUpdate(ctx, now); err != nil {
		t.Fatalf("SetLastUpdate returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}
