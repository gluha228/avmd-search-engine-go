package geo

import (
	"avmd-search-engine-go/internal/db"
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SQLCRepository struct {
	queries *db.Queries
}

func NewSQLCRepository(pool *pgxpool.Pool) *SQLCRepository {
	return &SQLCRepository{queries: db.New(pool)}
}

func (r *SQLCRepository) ListCountries(ctx context.Context) ([]Country, error) {
	countries, err := r.queries.ListCountries(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Country, len(countries))
	for i := range countries {
		result[i] = mapDBCountry(countries[i])
	}
	return result, nil
}

func (r *SQLCRepository) GetCountry(ctx context.Context, id int64) (*Country, error) {
	country, err := r.queries.GetCountry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBCountry(country)
	return &result, nil
}

func (r *SQLCRepository) CreateCountry(ctx context.Context, req CountryRequest) (*Country, error) {
	country, err := r.queries.CreateCountry(ctx, db.CreateCountryParams{
		NameRo: req.NameRo,
		NameRu: req.NameRu,
		NameEn: req.NameEn,
		Iso3:   req.ISO3,
		Iso2:   req.ISO2,
	})
	if err != nil {
		return nil, err
	}
	result := mapDBCountry(country)
	return &result, nil
}

func (r *SQLCRepository) UpdateCountry(ctx context.Context, id int64, req CountryRequest) (*Country, error) {
	country, err := r.queries.UpdateCountry(ctx, db.UpdateCountryParams{
		ID:     id,
		NameRo: req.NameRo,
		NameRu: req.NameRu,
		NameEn: req.NameEn,
		Iso3:   req.ISO3,
		Iso2:   req.ISO2,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBCountry(country)
	return &result, nil
}

func (r *SQLCRepository) DeleteCountry(ctx context.Context, id int64) error {
	rows, err := r.queries.DeleteCountry(ctx, id)
	return deletedOrError(rows, err)
}

func (r *SQLCRepository) ListCities(ctx context.Context) ([]City, error) {
	cities, err := r.queries.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]City, len(cities))
	for i := range cities {
		result[i] = mapDBCity(cities[i])
	}
	return result, nil
}

func (r *SQLCRepository) GetCity(ctx context.Context, id int64) (*City, error) {
	city, err := r.queries.GetCity(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBCity(city)
	return &result, nil
}

func (r *SQLCRepository) CreateCity(ctx context.Context, req CityRequest) (*City, error) {
	isCapital := false
	if req.IsCapital != nil {
		isCapital = *req.IsCapital
	}
	city, err := r.queries.CreateCity(ctx, db.CreateCityParams{
		CountryID:  req.CountryID,
		NameRo:     req.NameRo,
		NameRu:     req.NameRu,
		NameEn:     req.NameEn,
		IsCapital:  isCapital,
		Population: req.Population,
		Timezone:   req.Timezone,
	})
	if err != nil {
		return nil, err
	}
	result := mapDBCity(city)
	return &result, nil
}

func (r *SQLCRepository) UpdateCity(ctx context.Context, id int64, req CityRequest) (*City, error) {
	isCapital := false
	if req.IsCapital != nil {
		isCapital = *req.IsCapital
	}
	city, err := r.queries.UpdateCity(ctx, db.UpdateCityParams{
		ID:         id,
		CountryID:  req.CountryID,
		NameRo:     req.NameRo,
		NameRu:     req.NameRu,
		NameEn:     req.NameEn,
		IsCapital:  isCapital,
		Population: req.Population,
		Timezone:   req.Timezone,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBCity(city)
	return &result, nil
}

func (r *SQLCRepository) DeleteCity(ctx context.Context, id int64) error {
	rows, err := r.queries.DeleteCity(ctx, id)
	return deletedOrError(rows, err)
}

func (r *SQLCRepository) ListAirports(ctx context.Context) ([]Airport, error) {
	airports, err := r.queries.ListAirports(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Airport, len(airports))
	for i := range airports {
		result[i] = mapDBAirport(airports[i])
	}
	return result, nil
}

func (r *SQLCRepository) GetAirport(ctx context.Context, id int64) (*Airport, error) {
	airport, err := r.queries.GetAirport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBAirport(airport)
	return &result, nil
}

func (r *SQLCRepository) CreateAirport(ctx context.Context, req AirportRequest) (*Airport, error) {
	airport, err := r.queries.CreateAirport(ctx, db.CreateAirportParams{
		CityID:   req.CityID,
		IataCode: normalizeCodePtr(req.IATACode),
		IcaoCode: normalizeCodePtr(req.ICAOCode),
		Lat:      req.Lat,
		Lon:      req.Lon,
	})
	if err != nil {
		return nil, err
	}
	result := mapDBAirport(airport)
	return &result, nil
}

func (r *SQLCRepository) UpdateAirport(ctx context.Context, id int64, req AirportRequest) (*Airport, error) {
	airport, err := r.queries.UpdateAirport(ctx, db.UpdateAirportParams{
		ID:       id,
		CityID:   req.CityID,
		IataCode: normalizeCodePtr(req.IATACode),
		IcaoCode: normalizeCodePtr(req.ICAOCode),
		Lat:      req.Lat,
		Lon:      req.Lon,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := mapDBAirport(airport)
	return &result, nil
}

func (r *SQLCRepository) DeleteAirport(ctx context.Context, id int64) error {
	rows, err := r.queries.DeleteAirport(ctx, id)
	return deletedOrError(rows, err)
}

func (r *SQLCRepository) Dropdown(ctx context.Context, req CityDropdownRequest) ([]CityDropdown, error) {
	rows, err := r.queries.GetCitiesDropdown(ctx, db.GetCitiesDropdownParams{
		Locale:    normalizeLocale(req.Locale),
		Search:    strings.ToLower(strings.TrimSpace(req.Search)),
		LimitRows: int64(req.Limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]CityDropdown, len(rows))
	for i := range rows {
		airportCode := ""
		if rows[i].AirportCode != nil {
			airportCode = *rows[i].AirportCode
		}
		result[i] = CityDropdown{
			ID:          rows[i].ID,
			Name:        rows[i].Name,
			CountryName: rows[i].CountryName,
			AirportCode: airportCode,
		}
	}
	return result, nil
}

func mapDBCountry(country db.Country) Country {
	return Country{
		ID:     country.ID,
		NameRo: country.NameRo,
		NameRu: country.NameRu,
		NameEn: country.NameEn,
		ISO3:   country.Iso3,
		ISO2:   country.Iso2,
	}
}

func mapDBCity(city db.City) City {
	return City{
		ID:         city.ID,
		CountryID:  city.CountryID,
		NameRo:     city.NameRo,
		NameRu:     city.NameRu,
		NameEn:     city.NameEn,
		IsCapital:  city.IsCapital,
		Population: city.Population,
		Timezone:   city.Timezone,
	}
}

func mapDBAirport(airport db.Airport) Airport {
	return Airport{
		ID:       airport.ID,
		CityID:   airport.CityID,
		IATACode: airport.IataCode,
		ICAOCode: airport.IcaoCode,
		Lat:      airport.Lat,
		Lon:      airport.Lon,
	}
}

func deletedOrError(rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeCodePtr(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.ToUpper(strings.TrimSpace(*value))
	if normalized == "" {
		return nil
	}
	return &normalized
}
