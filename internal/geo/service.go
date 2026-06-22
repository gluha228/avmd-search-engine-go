package geo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidRequest = errors.New("invalid geo request")
	ErrNotFound       = errors.New("geo resource not found")
	iataPattern       = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Repository interface {
	ListCountries(ctx context.Context) ([]Country, error)
	GetCountry(ctx context.Context, id int64) (*Country, error)
	CreateCountry(ctx context.Context, req CountryRequest) (*Country, error)
	UpdateCountry(ctx context.Context, id int64, req CountryRequest) (*Country, error)
	DeleteCountry(ctx context.Context, id int64) error
	ListCities(ctx context.Context) ([]City, error)
	GetCity(ctx context.Context, id int64) (*City, error)
	CreateCity(ctx context.Context, req CityRequest) (*City, error)
	UpdateCity(ctx context.Context, id int64, req CityRequest) (*City, error)
	DeleteCity(ctx context.Context, id int64) error
	ListAirports(ctx context.Context) ([]Airport, error)
	GetAirport(ctx context.Context, id int64) (*Airport, error)
	CreateAirport(ctx context.Context, req AirportRequest) (*Airport, error)
	UpdateAirport(ctx context.Context, id int64, req AirportRequest) (*Airport, error)
	DeleteAirport(ctx context.Context, id int64) error
	Dropdown(ctx context.Context, req CityDropdownRequest) ([]CityDropdown, error)
}

type RouteProvider interface {
	IsKnownAirport(ctx context.Context, airportCode string) bool
	IsValidAirportRoute(ctx context.Context, originCode, destinationCode string) bool
}

type Service struct {
	repo          Repository
	routeProvider RouteProvider
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithRouteProvider(repo Repository, routeProvider RouteProvider) *Service {
	return &Service{repo: repo, routeProvider: routeProvider}
}

func (s *Service) ListCountries(ctx context.Context) ([]Country, error) {
	return s.repo.ListCountries(ctx)
}

func (s *Service) GetCountry(ctx context.Context, id int64) (*Country, error) {
	return s.repo.GetCountry(ctx, id)
}

func (s *Service) CreateCountry(ctx context.Context, req CountryRequest) (*Country, error) {
	if err := validateCountry(req); err != nil {
		return nil, err
	}
	return s.repo.CreateCountry(ctx, req)
}

func (s *Service) UpdateCountry(ctx context.Context, id int64, req CountryRequest) (*Country, error) {
	if err := validateCountry(req); err != nil {
		return nil, err
	}
	return s.repo.UpdateCountry(ctx, id, req)
}

func (s *Service) DeleteCountry(ctx context.Context, id int64) error {
	return s.repo.DeleteCountry(ctx, id)
}

func (s *Service) ListCities(ctx context.Context) ([]City, error) {
	return s.repo.ListCities(ctx)
}

func (s *Service) GetCity(ctx context.Context, id int64) (*City, error) {
	return s.repo.GetCity(ctx, id)
}

func (s *Service) CreateCity(ctx context.Context, req CityRequest) (*City, error) {
	if err := validateCity(req); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetCountry(ctx, req.CountryID); err != nil {
		return nil, err
	}
	return s.repo.CreateCity(ctx, req)
}

func (s *Service) UpdateCity(ctx context.Context, id int64, req CityRequest) (*City, error) {
	if err := validateCity(req); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetCountry(ctx, req.CountryID); err != nil {
		return nil, err
	}
	return s.repo.UpdateCity(ctx, id, req)
}

func (s *Service) DeleteCity(ctx context.Context, id int64) error {
	return s.repo.DeleteCity(ctx, id)
}

func (s *Service) ListAirports(ctx context.Context) ([]Airport, error) {
	return s.repo.ListAirports(ctx)
}

func (s *Service) GetAirport(ctx context.Context, id int64) (*Airport, error) {
	return s.repo.GetAirport(ctx, id)
}

func (s *Service) CreateAirport(ctx context.Context, req AirportRequest) (*Airport, error) {
	if err := validateAirport(req); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetCity(ctx, req.CityID); err != nil {
		return nil, err
	}
	return s.repo.CreateAirport(ctx, req)
}

func (s *Service) UpdateAirport(ctx context.Context, id int64, req AirportRequest) (*Airport, error) {
	if err := validateAirport(req); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetCity(ctx, req.CityID); err != nil {
		return nil, err
	}
	return s.repo.UpdateAirport(ctx, id, req)
}

func (s *Service) DeleteAirport(ctx context.Context, id int64) error {
	return s.repo.DeleteAirport(ctx, id)
}

func (s *Service) Dropdown(ctx context.Context, req CityDropdownRequest) ([]CityDropdown, error) {
	req.Search = strings.TrimSpace(req.Search)
	req.Locale = normalizeLocale(req.Locale)
	if req.Limit == 0 {
		req.Limit = 50
	}
	if len(req.Search) < 2 {
		return nil, fmt.Errorf("%w: search must contain at least 2 characters", ErrInvalidRequest)
	}
	if req.Limit < 5 || req.Limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 5 and 100", ErrInvalidRequest)
	}
	if req.OriginAirportCode != nil {
		origin := strings.ToUpper(strings.TrimSpace(*req.OriginAirportCode))
		if origin != "" && !iataPattern.MatchString(origin) {
			return nil, fmt.Errorf("%w: originAirportCode must be a 3-letter IATA code", ErrInvalidRequest)
		}
		if origin == "" {
			req.OriginAirportCode = nil
		} else {
			req.OriginAirportCode = &origin
		}
	}

	items, err := s.repo.Dropdown(ctx, req)
	if err != nil {
		return nil, err
	}
	filtered := make([]CityDropdown, 0, len(items))
	for _, item := range items {
		if !s.isReachableAirport(ctx, item.AirportCode) {
			continue
		}
		if req.OriginAirportCode != nil && !s.isValidRouteByAnyProvider(ctx, *req.OriginAirportCode, item.AirportCode) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func validateCountry(req CountryRequest) error {
	if isBlank(req.NameRo) || isBlank(req.NameRu) || isBlank(req.NameEn) {
		return fmt.Errorf("%w: localized country names are required", ErrInvalidRequest)
	}
	if len(strings.TrimSpace(req.ISO3)) != 3 {
		return fmt.Errorf("%w: iso3 must be exactly 3 characters", ErrInvalidRequest)
	}
	if len(strings.TrimSpace(req.ISO2)) != 2 {
		return fmt.Errorf("%w: iso2 must be exactly 2 characters", ErrInvalidRequest)
	}
	return nil
}

func validateCity(req CityRequest) error {
	if req.CountryID <= 0 {
		return fmt.Errorf("%w: country_id is required", ErrInvalidRequest)
	}
	if isBlank(req.NameRo) || isBlank(req.NameRu) || isBlank(req.NameEn) {
		return fmt.Errorf("%w: localized city names are required", ErrInvalidRequest)
	}
	return nil
}

func validateAirport(req AirportRequest) error {
	if req.CityID <= 0 {
		return fmt.Errorf("%w: city_id is required", ErrInvalidRequest)
	}
	return nil
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func normalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if strings.HasPrefix(locale, "ru") {
		return "ru"
	}
	if strings.HasPrefix(locale, "ro") {
		return "ro"
	}
	return "en"
}

func (s *Service) isReachableAirport(ctx context.Context, airportCode string) bool {
	return isReachableAirport(airportCode) ||
		(s.routeProvider != nil && s.routeProvider.IsKnownAirport(ctx, airportCode))
}

func (s *Service) isValidRouteByAnyProvider(ctx context.Context, origin, destination string) bool {
	return isValidRoute(origin, destination) ||
		(s.routeProvider != nil && s.routeProvider.IsValidAirportRoute(ctx, origin, destination))
}
