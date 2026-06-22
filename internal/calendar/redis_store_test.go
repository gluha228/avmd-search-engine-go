package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
)

func TestRedisPriceStoreUsesJavaCompatiblePriceValue(t *testing.T) {
	client, mock := redismock.NewClientMock()
	store := NewRedisPriceStore(client, time.Hour)
	ctx := context.Background()
	date := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectGet("flight:price:KIV:OTP:2026-07-01").RedisNil()
	mock.ExpectSet("flight:price:KIV:OTP:2026-07-01", "123.45", time.Hour).SetVal("OK")
	if err := store.SetMinPriceIfLower(ctx, "kiv", "otp", date, PriceEntry{Price: 123.45}); err != nil {
		t.Fatalf("SetMinPriceIfLower returned error: %v", err)
	}

	mock.ExpectGet("flight:price:KIV:OTP:2026-07-01").SetVal("123.45")
	entry, err := store.GetMinPrice(ctx, "KIV", "OTP", date)
	if err != nil {
		t.Fatalf("GetMinPrice returned error: %v", err)
	}
	if entry == nil || entry.Price != 123.45 {
		t.Fatalf("expected price 123.45, got %+v", entry)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}
