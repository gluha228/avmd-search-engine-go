package session

import (
	"avmd-search-engine-go/internal/flights"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
)

func TestRedisStoreSaveSavesSessionWithTTL(t *testing.T) {
	db, mock := redismock.NewClientMock()
	store := NewRedisStore(db, 24*time.Hour, nil)
	session := flights.FlightSearchSession{
		TFRoutingID: "RID",
		TFOffers: []flights.Offer{
			{OfferID: "OUT1", CurrencyCode: "EUR", Price: 100},
		},
	}
	payload, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	mock.ExpectSet("flight:session:search-1", payload, 24*time.Hour).SetVal("OK")

	if err := store.Save(context.Background(), "search-1", session); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations were not met: %v", err)
	}
}
