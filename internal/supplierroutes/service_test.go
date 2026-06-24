package supplierroutes

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTFClient struct {
	suppliers []string
	routes    map[string]*travelfusion.SupplierRoutesResult
	calls     int
}

type fakeStore struct {
	airportRoutes []string
	cityRoutes    []string
	knownAirports []string
	lastUpdate    *time.Time
	err           error
}

func (c *fakeTFClient) GetBranchSupplierList(context.Context) ([]string, error) {
	c.calls++
	return c.suppliers, nil
}

func (c *fakeTFClient) ListSupplierRoutes(_ context.Context, supplier string, _ bool) (*travelfusion.SupplierRoutesResult, error) {
	return c.routes[supplier], nil
}

func (s *fakeStore) IsValidAirportRoute(_ context.Context, originCode, destinationCode string) (bool, error) {
	return contains(s.airportRoutes, originCode+destinationCode), nil
}

func (s *fakeStore) IsKnownAirport(_ context.Context, airportCode string) (bool, error) {
	return contains(s.knownAirports, airportCode), nil
}

func (s *fakeStore) IsValidCityRoute(_ context.Context, originCode, destinationCode string) (bool, error) {
	return contains(s.cityRoutes, originCode+destinationCode), nil
}

func (s *fakeStore) ReplaceRoutes(_ context.Context, airportRoutes, cityRoutes, knownAirports []string) error {
	s.airportRoutes = airportRoutes
	s.cityRoutes = cityRoutes
	s.knownAirports = knownAirports
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

func TestRefreshCollectsRoutesAndKnownAirports(t *testing.T) {
	store := &fakeStore{}
	now := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	service := NewService(&fakeTFClient{
		suppliers: []string{"s1", "s2"},
		routes: map[string]*travelfusion.SupplierRoutesResult{
			"s1": {AirportRoutes: []string{"OTPCLJ", "CLJOTP"}, CityRoutes: []string{"LONPAR"}},
			"s2": {AirportRoutes: []string{"OTPTLV"}},
		},
	}, store, Config{UpdateTime: "04:00"}, nil)
	service.now = func() time.Time { return now }

	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if !contains(store.airportRoutes, "OTPCLJ") || !contains(store.airportRoutes, "OTPTLV") {
		t.Fatalf("expected airport routes to be cached, got %+v", store.airportRoutes)
	}
	if !contains(store.knownAirports, "OTP") || !contains(store.knownAirports, "TLV") {
		t.Fatalf("expected known airports to be cached, got %+v", store.knownAirports)
	}
	if store.lastUpdate == nil || !store.lastUpdate.Equal(now) {
		t.Fatalf("expected last update %v, got %v", now, store.lastUpdate)
	}
}

func TestNeedsRefreshDoesNotRefreshAgainBeforeTodayUpdateTime(t *testing.T) {
	lastUpdate := time.Date(2026, 7, 1, 0, 10, 0, 0, time.UTC)
	service := NewService(&fakeTFClient{}, &fakeStore{lastUpdate: &lastUpdate}, Config{UpdateTime: "04:00"}, nil)
	service.now = func() time.Time { return time.Date(2026, 7, 1, 0, 20, 0, 0, time.UTC) }

	needed, err := service.NeedsRefresh(context.Background())
	if err != nil {
		t.Fatalf("NeedsRefresh returned error: %v", err)
	}
	if needed {
		t.Fatal("expected refresh not to be needed before today's scheduled update time")
	}
}

func TestRouteLookupNormalizesCodes(t *testing.T) {
	service := NewService(&fakeTFClient{}, &fakeStore{
		airportRoutes: []string{"OTPCLJ"},
		knownAirports: []string{"OTP"},
	}, Config{}, nil)

	if !service.IsKnownAirport(context.Background(), "otp") {
		t.Fatal("expected known airport lookup to normalize code")
	}
	if !service.IsValidAirportRoute(context.Background(), "otp", "clj") {
		t.Fatal("expected route lookup to normalize codes")
	}
}

func TestRefreshIfNeededDoesNotCallTFWhenStatusCheckFails(t *testing.T) {
	client := &fakeTFClient{suppliers: []string{"s1"}}
	service := NewService(client, &fakeStore{err: errors.New("redis unavailable")}, Config{UpdateTime: "04:00"}, nil)

	if err := service.RefreshIfNeeded(context.Background()); err == nil {
		t.Fatal("expected RefreshIfNeeded to return status check error")
	}
	if client.calls != 0 {
		t.Fatalf("expected TF not to be called, got %d calls", client.calls)
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
