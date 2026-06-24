package currencies

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTFClient struct {
	currencies map[string]travelfusion.Currency
	err        error
	calls      int
}

type fakeStore struct {
	rates      map[string]float64
	lastUpdate *time.Time
	err        error
}

func (c *fakeTFClient) GetCurrencies(context.Context) (map[string]travelfusion.Currency, error) {
	c.calls++
	return c.currencies, c.err
}

func (s *fakeStore) GetRate(_ context.Context, currencyCode string) (float64, error) {
	rate, ok := s.rates[currencyCode]
	if !ok {
		return 0, ErrRateUnavailable
	}
	return rate, nil
}

func (s *fakeStore) SetRate(_ context.Context, currencyCode string, rate float64) error {
	s.rates[currencyCode] = rate
	return nil
}

func (s *fakeStore) LastUpdate(context.Context) (*time.Time, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.lastUpdate, nil
}

func (s *fakeStore) SetLastUpdate(_ context.Context, t time.Time) error {
	s.lastUpdate = &t
	return nil
}

func TestGetRateUsesUSDRelativeRates(t *testing.T) {
	service := NewService(&fakeTFClient{}, &fakeStore{rates: map[string]float64{
		"EUR": 0.8,
		"GBP": 0.5,
	}}, Config{UpdateTime: "03:00"}, nil)

	rate, err := service.GetRate(context.Background(), "GBP", "EUR")
	if err != nil {
		t.Fatalf("GetRate returned error: %v", err)
	}
	if rate != 1.6 {
		t.Fatalf("expected 1.6, got %v", rate)
	}
}

func TestConvertRoundsToTwoDecimals(t *testing.T) {
	service := NewService(&fakeTFClient{}, &fakeStore{rates: map[string]float64{
		"EUR": 0.93,
	}}, Config{UpdateTime: "03:00"}, nil)

	converted, err := service.Convert(context.Background(), 10.005, "USD", "EUR")
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if converted != 9.3 {
		t.Fatalf("expected 9.3, got %v", converted)
	}
}

func TestRefreshIfNeededStoresRatesAndTimestamp(t *testing.T) {
	store := &fakeStore{rates: map[string]float64{}}
	service := NewService(&fakeTFClient{currencies: map[string]travelfusion.Currency{
		"EUR": {Code: "EUR", USDRate: 0.9},
	}}, store, Config{UpdateTime: "03:00"}, nil)
	now := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if err := service.RefreshIfNeeded(context.Background()); err != nil {
		t.Fatalf("RefreshIfNeeded returned error: %v", err)
	}
	if store.rates["EUR"] != 0.9 || store.rates["USD"] != 1 {
		t.Fatalf("expected EUR and USD rates to be stored, got %+v", store.rates)
	}
	if store.lastUpdate == nil || !store.lastUpdate.Equal(now) {
		t.Fatalf("expected last update %v, got %v", now, store.lastUpdate)
	}
}

func TestNeedsRefreshUsesTodayUpdateTime(t *testing.T) {
	lastUpdate := time.Date(2026, 7, 1, 2, 59, 0, 0, time.UTC)
	service := NewService(&fakeTFClient{}, &fakeStore{
		rates:      map[string]float64{},
		lastUpdate: &lastUpdate,
	}, Config{UpdateTime: "03:00"}, nil)
	service.now = func() time.Time { return time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC) }

	needed, err := service.NeedsRefresh(context.Background())
	if err != nil {
		t.Fatalf("NeedsRefresh returned error: %v", err)
	}
	if !needed {
		t.Fatal("expected refresh to be needed")
	}
}

func TestNeedsRefreshDoesNotRefreshAgainBeforeTodayUpdateTime(t *testing.T) {
	lastUpdate := time.Date(2026, 7, 1, 0, 10, 0, 0, time.UTC)
	service := NewService(&fakeTFClient{}, &fakeStore{
		rates:      map[string]float64{},
		lastUpdate: &lastUpdate,
	}, Config{UpdateTime: "03:00"}, nil)
	service.now = func() time.Time { return time.Date(2026, 7, 1, 0, 14, 0, 0, time.UTC) }

	needed, err := service.NeedsRefresh(context.Background())
	if err != nil {
		t.Fatalf("NeedsRefresh returned error: %v", err)
	}
	if needed {
		t.Fatal("expected refresh not to be needed before today's scheduled update time")
	}
}

func TestRefreshIfNeededDoesNotCallTFOnRepeatedRestartBeforeUpdateTime(t *testing.T) {
	lastUpdate := time.Date(2026, 7, 1, 0, 10, 0, 0, time.UTC)
	client := &fakeTFClient{currencies: map[string]travelfusion.Currency{
		"EUR": {Code: "EUR", USDRate: 0.9},
	}}
	service := NewService(client, &fakeStore{
		rates:      map[string]float64{},
		lastUpdate: &lastUpdate,
	}, Config{UpdateTime: "03:00"}, nil)
	service.now = func() time.Time { return time.Date(2026, 7, 1, 0, 14, 0, 0, time.UTC) }

	if err := service.RefreshIfNeeded(context.Background()); err != nil {
		t.Fatalf("RefreshIfNeeded returned error: %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("expected TF not to be called, got %d calls", client.calls)
	}
}

func TestRefreshIfNeededDoesNotCallTFWhenStatusCheckFails(t *testing.T) {
	client := &fakeTFClient{currencies: map[string]travelfusion.Currency{
		"EUR": {Code: "EUR", USDRate: 0.9},
	}}
	service := NewService(client, &fakeStore{
		rates: map[string]float64{},
		err:   errors.New("redis unavailable"),
	}, Config{UpdateTime: "03:00"}, nil)

	if err := service.RefreshIfNeeded(context.Background()); err == nil {
		t.Fatal("expected RefreshIfNeeded to return status check error")
	}
	if client.calls != 0 {
		t.Fatalf("expected TF not to be called, got %d calls", client.calls)
	}
}

func TestStartAcceptsJavaStyleCronExpression(t *testing.T) {
	lastUpdate := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	service := NewService(&fakeTFClient{}, &fakeStore{
		rates:      map[string]float64{},
		lastUpdate: &lastUpdate,
	}, Config{UpdateCron: "0 0 3 * * ?", UpdateTime: "03:00"}, nil)
	service.now = func() time.Time { return time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC) }

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	service.Stop()
}
