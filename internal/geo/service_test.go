package geo

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	country *Country
	city    *City
	items   []CityDropdown
	created CityRequest
}

func (r *fakeRepo) ListCountries(context.Context) ([]Country, error) { return nil, nil }
func (r *fakeRepo) GetCountry(context.Context, int64) (*Country, error) {
	if r.country == nil {
		return nil, ErrNotFound
	}
	return r.country, nil
}
func (r *fakeRepo) CreateCountry(context.Context, CountryRequest) (*Country, error) { return nil, nil }
func (r *fakeRepo) UpdateCountry(context.Context, int64, CountryRequest) (*Country, error) {
	return nil, nil
}
func (r *fakeRepo) DeleteCountry(context.Context, int64) error { return nil }
func (r *fakeRepo) ListCities(context.Context) ([]City, error) { return nil, nil }
func (r *fakeRepo) GetCity(context.Context, int64) (*City, error) {
	if r.city == nil {
		return nil, ErrNotFound
	}
	return r.city, nil
}
func (r *fakeRepo) CreateCity(_ context.Context, req CityRequest) (*City, error) {
	r.created = req
	return &City{
		ID:        1,
		CountryID: req.CountryID,
		NameRo:    req.NameRo,
		NameRu:    req.NameRu,
		NameEn:    req.NameEn,
	}, nil
}
func (r *fakeRepo) UpdateCity(context.Context, int64, CityRequest) (*City, error) { return nil, nil }
func (r *fakeRepo) DeleteCity(context.Context, int64) error                       { return nil }
func (r *fakeRepo) ListAirports(context.Context) ([]Airport, error)               { return nil, nil }
func (r *fakeRepo) GetAirport(context.Context, int64) (*Airport, error)           { return nil, nil }
func (r *fakeRepo) CreateAirport(context.Context, AirportRequest) (*Airport, error) {
	return nil, nil
}
func (r *fakeRepo) UpdateAirport(context.Context, int64, AirportRequest) (*Airport, error) {
	return nil, nil
}
func (r *fakeRepo) DeleteAirport(context.Context, int64) error { return nil }
func (r *fakeRepo) Dropdown(context.Context, CityDropdownRequest) ([]CityDropdown, error) {
	return r.items, nil
}

func TestCreateCityDefaultsIsCapitalToFalse(t *testing.T) {
	repo := &fakeRepo{country: &Country{ID: 1}}
	service := NewService(repo)

	city, err := service.CreateCity(context.Background(), CityRequest{
		CountryID: 1,
		NameRo:    "Chisinau",
		NameRu:    "Kishinev",
		NameEn:    "Chisinau",
	})
	if err != nil {
		t.Fatalf("CreateCity returned error: %v", err)
	}
	if city.IsCapital {
		t.Fatal("expected is_capital to default to false")
	}
	if repo.created.IsCapital != nil {
		t.Fatal("expected request is_capital to remain nil before store defaulting")
	}
}

func TestCreateCityRequiresExistingCountry(t *testing.T) {
	service := NewService(&fakeRepo{})

	_, err := service.CreateCity(context.Background(), CityRequest{
		CountryID: 1,
		NameRo:    "Chisinau",
		NameRu:    "Kishinev",
		NameEn:    "Chisinau",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDropdownValidatesSearchAndLimit(t *testing.T) {
	service := NewService(&fakeRepo{})

	_, err := service.Dropdown(context.Background(), CityDropdownRequest{Search: "a", Limit: 50})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for short search, got %v", err)
	}

	_, err = service.Dropdown(context.Background(), CityDropdownRequest{Search: "chi", Limit: 101})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for limit, got %v", err)
	}
}

func TestDropdownFiltersRoutes(t *testing.T) {
	origin := "OTP"
	service := NewService(&fakeRepo{items: []CityDropdown{
		{ID: 1, AirportCode: "CLJ"},
		{ID: 2, AirportCode: "ZZZ"},
	}})

	result, err := service.Dropdown(context.Background(), CityDropdownRequest{
		Search:            "clu",
		OriginAirportCode: &origin,
		Limit:             50,
	})
	if err != nil {
		t.Fatalf("Dropdown returned error: %v", err)
	}
	if len(result) != 1 || result[0].AirportCode != "CLJ" {
		t.Fatalf("expected only valid OTP -> CLJ route, got %+v", result)
	}
}
