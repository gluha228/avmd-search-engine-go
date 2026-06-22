package calendar

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"testing"
	"time"
)

type fakePriceStore struct {
	entries map[string]PriceEntry
}

func (s *fakePriceStore) GetMinPrice(_ context.Context, origin, destination string, date time.Time) (*PriceEntry, error) {
	entry, ok := s.entries[origin+":"+destination+":"+date.Format(time.DateOnly)]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func (s *fakePriceStore) SetMinPriceIfLower(_ context.Context, origin, destination string, date time.Time, entry PriceEntry) error {
	key := origin + ":" + destination + ":" + date.Format(time.DateOnly)
	if current, ok := s.entries[key]; !ok || entry.Price < current.Price {
		s.entries[key] = entry
	}
	return nil
}

func TestGetCalendarReturnsCachedDays(t *testing.T) {
	store := &fakePriceStore{entries: map[string]PriceEntry{
		"KIV:OTP:2026-07-01": {Price: 100, CurrencyCode: "EUR"},
	}}
	service := NewService(store, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.GetCalendar(context.Background(), Request{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DateFrom:             time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		DateTo:               time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GetCalendar returned error: %v", err)
	}
	if len(resp.Calendar) != 1 || resp.Calendar[0].Date != "2026-07-01" || resp.Calendar[0].Price != 100 {
		t.Fatalf("unexpected calendar response: %+v", resp.Calendar)
	}
}

func TestCacheFlightsKeepsMinimumPricePerDate(t *testing.T) {
	store := &fakePriceStore{entries: map[string]PriceEntry{}}
	service := NewService(store, nil)
	date := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)

	err := service.CacheFlights(context.Background(), "KIV", "OTP", []travelfusion.Flight{
		{DepartureTime: date, Price: 200, Currency: "EUR"},
		{DepartureTime: date.Add(2 * time.Hour), Price: 150, Currency: "EUR"},
		{DepartureTime: date.AddDate(0, 0, 1), Price: 300, Currency: "EUR"},
	})
	if err != nil {
		t.Fatalf("CacheFlights returned error: %v", err)
	}
	if got := store.entries["KIV:OTP:2026-07-01"].Price; got != 150 {
		t.Fatalf("expected min price 150 for 2026-07-01, got %v", got)
	}
	if got := store.entries["KIV:OTP:2026-07-02"].Price; got != 300 {
		t.Fatalf("expected price 300 for 2026-07-02, got %v", got)
	}
}
