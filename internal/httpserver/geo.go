package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/geo"
	"context"
	"errors"
	"net/http"
	"strings"
)

func (s *HttpServer) ListCountries(ctx context.Context, _ api.ListCountriesRequestObject) (api.ListCountriesResponseObject, error) {
	countries, err := s.geoService.ListCountries(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListCountries200JSONResponse(mapCountries(countries)), nil
}

func (s *HttpServer) GetCountry(ctx context.Context, request api.GetCountryRequestObject) (api.GetCountryResponseObject, error) {
	country, err := s.geoService.GetCountry(ctx, request.Id)
	if errors.Is(err, geo.ErrNotFound) {
		return api.GetCountry404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "country not found"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetCountry200JSONResponse(mapCountry(*country)), nil
}

func (s *HttpServer) CreateCountry(ctx context.Context, request api.CreateCountryRequestObject) (api.CreateCountryResponseObject, error) {
	country, err := s.geoService.CreateCountry(ctx, mapCountryRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.CreateCountry400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.CreateCountry500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.CreateCountry201JSONResponse(mapCountry(*country)), nil
}

func (s *HttpServer) UpdateCountry(ctx context.Context, request api.UpdateCountryRequestObject) (api.UpdateCountryResponseObject, error) {
	country, err := s.geoService.UpdateCountry(ctx, request.Id, mapCountryRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.UpdateCountry400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, geo.ErrNotFound) {
		return api.UpdateCountry404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "country not found"}}, nil
	}
	if err != nil {
		return api.UpdateCountry500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.UpdateCountry200JSONResponse(mapCountry(*country)), nil
}

func (s *HttpServer) DeleteCountry(ctx context.Context, request api.DeleteCountryRequestObject) (api.DeleteCountryResponseObject, error) {
	if err := s.geoService.DeleteCountry(ctx, request.Id); errors.Is(err, geo.ErrNotFound) {
		return api.DeleteCountry404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "country not found"}}, nil
	} else if err != nil {
		return nil, err
	}
	return api.DeleteCountry204Response{}, nil
}

func mapCountryRequest(req api.CountryRequest) geo.CountryRequest {
	return geo.CountryRequest{NameRo: req.NameRo, NameRu: req.NameRu, NameEn: req.NameEn, ISO3: req.Iso3, ISO2: req.Iso2}
}

func mapCountry(country geo.Country) api.Country {
	return api.Country{Id: country.ID, NameRo: country.NameRo, NameRu: country.NameRu, NameEn: country.NameEn, Iso3: country.ISO3, Iso2: country.ISO2}
}

func mapCountries(countries []geo.Country) []api.Country {
	result := make([]api.Country, len(countries))
	for i := range countries {
		result[i] = mapCountry(countries[i])
	}
	return result
}

func (s *HttpServer) ListCities(ctx context.Context, _ api.ListCitiesRequestObject) (api.ListCitiesResponseObject, error) {
	cities, err := s.geoService.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListCities200JSONResponse(mapCities(cities)), nil
}

func (s *HttpServer) GetCity(ctx context.Context, request api.GetCityRequestObject) (api.GetCityResponseObject, error) {
	city, err := s.geoService.GetCity(ctx, request.Id)
	if errors.Is(err, geo.ErrNotFound) {
		return api.GetCity404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "city not found"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetCity200JSONResponse(mapCity(*city)), nil
}

func (s *HttpServer) CreateCity(ctx context.Context, request api.CreateCityRequestObject) (api.CreateCityResponseObject, error) {
	city, err := s.geoService.CreateCity(ctx, mapCityRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.CreateCity400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.CreateCity500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.CreateCity201JSONResponse(mapCity(*city)), nil
}

func (s *HttpServer) UpdateCity(ctx context.Context, request api.UpdateCityRequestObject) (api.UpdateCityResponseObject, error) {
	city, err := s.geoService.UpdateCity(ctx, request.Id, mapCityRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.UpdateCity400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, geo.ErrNotFound) {
		return api.UpdateCity404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "city not found"}}, nil
	}
	if err != nil {
		return api.UpdateCity500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.UpdateCity200JSONResponse(mapCity(*city)), nil
}

func (s *HttpServer) DeleteCity(ctx context.Context, request api.DeleteCityRequestObject) (api.DeleteCityResponseObject, error) {
	if err := s.geoService.DeleteCity(ctx, request.Id); errors.Is(err, geo.ErrNotFound) {
		return api.DeleteCity404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "city not found"}}, nil
	} else if err != nil {
		return nil, err
	}
	return api.DeleteCity204Response{}, nil
}

func (s *HttpServer) GetCitiesDropdown(ctx context.Context, request api.GetCitiesDropdownRequestObject) (api.GetCitiesDropdownResponseObject, error) {
	limit := 50
	if request.Params.Limit != nil {
		limit = int(*request.Params.Limit)
	}
	dropdowns, err := s.geoService.Dropdown(ctx, geo.CityDropdownRequest{
		Search:            request.Params.Search,
		OriginAirportCode: (*string)(request.Params.OriginAirportCode),
		Limit:             limit,
		Locale:            localeFromContext(ctx),
	})
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.GetCitiesDropdown400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.GetCitiesDropdown500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetCitiesDropdown200JSONResponse(mapCityDropdowns(dropdowns)), nil
}

func mapCityRequest(req api.CityRequest) geo.CityRequest {
	return geo.CityRequest{
		CountryID:  req.CountryId,
		NameRo:     req.NameRo,
		NameRu:     req.NameRu,
		NameEn:     req.NameEn,
		IsCapital:  req.IsCapital,
		Population: req.Population,
		Timezone:   req.Timezone,
	}
}

func mapCity(city geo.City) api.City {
	return api.City{
		Id:         city.ID,
		CountryId:  city.CountryID,
		NameRo:     city.NameRo,
		NameRu:     city.NameRu,
		NameEn:     city.NameEn,
		IsCapital:  city.IsCapital,
		Population: city.Population,
		Timezone:   city.Timezone,
	}
}

func mapCities(cities []geo.City) []api.City {
	result := make([]api.City, len(cities))
	for i := range cities {
		result[i] = mapCity(cities[i])
	}
	return result
}

func (s *HttpServer) ListAirports(ctx context.Context, _ api.ListAirportsRequestObject) (api.ListAirportsResponseObject, error) {
	airports, err := s.geoService.ListAirports(ctx)
	if err != nil {
		return nil, err
	}
	return api.ListAirports200JSONResponse(mapAirports(airports)), nil
}

func (s *HttpServer) GetAirport(ctx context.Context, request api.GetAirportRequestObject) (api.GetAirportResponseObject, error) {
	airport, err := s.geoService.GetAirport(ctx, request.Id)
	if errors.Is(err, geo.ErrNotFound) {
		return api.GetAirport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "airport not found"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.GetAirport200JSONResponse(mapAirport(*airport)), nil
}

func (s *HttpServer) CreateAirport(ctx context.Context, request api.CreateAirportRequestObject) (api.CreateAirportResponseObject, error) {
	airport, err := s.geoService.CreateAirport(ctx, mapAirportRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.CreateAirport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.CreateAirport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.CreateAirport201JSONResponse(mapAirport(*airport)), nil
}

func (s *HttpServer) UpdateAirport(ctx context.Context, request api.UpdateAirportRequestObject) (api.UpdateAirportResponseObject, error) {
	airport, err := s.geoService.UpdateAirport(ctx, request.Id, mapAirportRequest(*request.Body))
	if errors.Is(err, geo.ErrInvalidRequest) {
		return api.UpdateAirport400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, geo.ErrNotFound) {
		return api.UpdateAirport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "airport not found"}}, nil
	}
	if err != nil {
		return api.UpdateAirport500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.UpdateAirport200JSONResponse(mapAirport(*airport)), nil
}

func (s *HttpServer) DeleteAirport(ctx context.Context, request api.DeleteAirportRequestObject) (api.DeleteAirportResponseObject, error) {
	if err := s.geoService.DeleteAirport(ctx, request.Id); errors.Is(err, geo.ErrNotFound) {
		return api.DeleteAirport404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: "airport not found"}}, nil
	} else if err != nil {
		return nil, err
	}
	return api.DeleteAirport204Response{}, nil
}

func mapAirportRequest(req api.AirportRequest) geo.AirportRequest {
	return geo.AirportRequest{
		CityID:   req.CityId,
		IATACode: req.IataCode,
		ICAOCode: req.IcaoCode,
		Lat:      req.Lat,
		Lon:      req.Lon,
	}
}

func mapAirport(airport geo.Airport) api.Airport {
	return api.Airport{
		Id:       airport.ID,
		CityId:   airport.CityID,
		IataCode: airport.IATACode,
		IcaoCode: airport.ICAOCode,
		Lat:      airport.Lat,
		Lon:      airport.Lon,
	}
}

func mapAirports(airports []geo.Airport) []api.Airport {
	result := make([]api.Airport, len(airports))
	for i := range airports {
		result[i] = mapAirport(airports[i])
	}
	return result
}

func localeFromContext(ctx context.Context) string {
	req, ok := ctx.Value(requestContextKey{}).(*http.Request)
	if !ok || req == nil {
		return "en"
	}
	return strings.Split(req.Header.Get("Accept-Language"), ",")[0]
}

func mapCityDropdowns(dropdowns []geo.CityDropdown) []api.CityDropdown {
	result := make([]api.CityDropdown, len(dropdowns))
	for i := range dropdowns {
		result[i] = api.CityDropdown{
			Id:          dropdowns[i].ID,
			Name:        dropdowns[i].Name,
			CountryName: dropdowns[i].CountryName,
			AirportCode: dropdowns[i].AirportCode,
		}
	}
	return result
}
