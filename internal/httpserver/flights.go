package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	flightsearch "avmd-search-engine-go/internal/flights/search"
	flightsession "avmd-search-engine-go/internal/flights/session"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func (s *HttpServer) SearchFlights(
	ctx context.Context,
	request api.SearchFlightsRequestObject,
) (api.SearchFlightsResponseObject, error) {
	if err := s.validator.Struct(request.Params); err != nil {
		return api.SearchFlights400JSONResponse{Message: err.Error()}, nil
	}

	serviceReq, err := mapSearchRequest(request.Params)
	if err != nil {
		return api.SearchFlights400JSONResponse{Message: err.Error()}, nil
	}

	if err := s.searchService.Validate(serviceReq); errors.Is(err, flightsearch.ErrInvalidRequest) {
		return api.SearchFlights400JSONResponse{Message: err.Error()}, nil
	} else if err != nil {
		return api.SearchFlights500JSONResponse{Message: err.Error()}, nil
	}

	return searchFlightsSSEResponse{
		ctx:     ctx,
		service: s.searchService,
		request: serviceReq,
	}, nil
}

func mapSearchRequest(params api.SearchFlightsParams) (flightsearch.SearchRequest, error) {
	req := flightsearch.SearchRequest{
		DepartureAirportCode:                params.DepartureAirportCode,
		ArrivalAirportCode:                  params.ArrivalAirportCode,
		DepartureDate:                       params.DepartureDate.Time,
		AdultCount:                          int(params.AdultCount),
		ChildCount:                          intValue(params.ChildCount),
		InfantCount:                         intValue(params.InfantCount),
		MinPrice:                            floatPtr(params.MinPrice),
		MaxPrice:                            floatPtr(params.MaxPrice),
		MinSegments:                         intPtrFromParam(params.MinSegments),
		MaxSegments:                         intPtrFromParam(params.MaxSegments),
		MinTotalDurationMinutes:             intPtrFromParam(params.MinTotalDurationMinutes),
		MaxTotalDurationMinutes:             intPtrFromParam(params.MaxTotalDurationMinutes),
		MinIndividualSegmentDurationMinutes: intPtrFromParam(params.MinIndividualSegmentDurationMinutes),
		MaxIndividualSegmentDurationMinutes: intPtrFromParam(params.MaxIndividualSegmentDurationMinutes),
		MinLayoverMinutes:                   intPtrFromParam(params.MinLayoverMinutes),
		MaxLayoverMinutes:                   intPtrFromParam(params.MaxLayoverMinutes),
	}
	if params.ReturnDate != nil {
		returnDate := params.ReturnDate.Time
		req.ReturnDate = &returnDate
	}

	var err error
	if req.DepartureOutboundFrom, err = parseOptionalClock(params.DepartureOutboundFrom); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("departureOutboundFrom: %w", err)
	}
	if req.DepartureOutboundTo, err = parseOptionalClock(params.DepartureOutboundTo); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("departureOutboundTo: %w", err)
	}
	if req.ArrivalOutboundFrom, err = parseOptionalClock(params.ArrivalOutboundFrom); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("arrivalOutboundFrom: %w", err)
	}
	if req.ArrivalOutboundTo, err = parseOptionalClock(params.ArrivalOutboundTo); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("arrivalOutboundTo: %w", err)
	}
	if req.DepartureInboundFrom, err = parseOptionalClock(params.DepartureInboundFrom); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("departureInboundFrom: %w", err)
	}
	if req.DepartureInboundTo, err = parseOptionalClock(params.DepartureInboundTo); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("departureInboundTo: %w", err)
	}
	if req.ArrivalInboundFrom, err = parseOptionalClock(params.ArrivalInboundFrom); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("arrivalInboundFrom: %w", err)
	}
	if req.ArrivalInboundTo, err = parseOptionalClock(params.ArrivalInboundTo); err != nil {
		return flightsearch.SearchRequest{}, fmt.Errorf("arrivalInboundTo: %w", err)
	}
	return req, nil
}

func parseOptionalClock[T ~string](value *T) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", string(*value))
	if err != nil {
		return nil, fmt.Errorf("must use HH:mm format")
	}
	return &parsed, nil
}

func intValue(value *int32) int {
	if value == nil {
		return 0
	}
	return int(*value)
}

func intPtrFromParam(value *int32) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

func floatPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return value
}

type searchFlightsSSEResponse struct {
	ctx     context.Context
	service *flightsearch.Service
	request flightsearch.SearchRequest
}

func (response searchFlightsSSEResponse) VisitSearchFlightsResponse(w http.ResponseWriter) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is not supported by response writer")
	}

	searchID, err := response.service.CreateSession(response.ctx, response.request)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	if err := writeSSE(w, flusher, "search_id", sseSearchID{SearchID: searchID}); err != nil {
		return err
	}

	locale := localeFromContext(response.ctx)
	for update := range response.service.SearchIntoSessionStream(response.ctx, searchID, response.request) {
		if update.Err != nil {
			_ = writeSSE(w, flusher, "error", map[string]string{"message": update.Err.Error()})
			return nil
		}
		if len(update.Offers) == 0 {
			continue
		}
		enrichedOffers, err := response.service.EnrichSearchOffers(response.ctx, update.Offers, locale)
		if err != nil {
			_ = writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
			return nil
		}
		if err := writeSSE(w, flusher, "offers", mapOffers(enrichedOffers)); err != nil {
			return err
		}
	}

	return writeSSEString(w, flusher, "done", "")
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeSSEString(w http.ResponseWriter, flusher http.Flusher, event string, data string) error {
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

type sseOffer struct {
	OfferID         string             `json:"offer_id"`
	OutboundFlight  sseFlight          `json:"outbound_flight"`
	InboundFlight   *sseFlight         `json:"inbound_flight,omitempty"`
	CurrencyCode    string             `json:"currency_code"`
	FareBand        sseFareBand        `json:"fare_band"`
	Price           float64            `json:"price"`
	PassengerPrices ssePassengerPrices `json:"passenger_prices"`
}

type sseSearchID struct {
	SearchID string `json:"searchId"`
}

type ssePassengerPrices struct {
	Adults   []float64 `json:"adults"`
	Children []float64 `json:"children"`
	Infants  []float64 `json:"infants"`
}

type sseFareBand struct {
	Name     string   `json:"name"`
	Features []string `json:"features"`
}

type sseFlight struct {
	DepartureFlightAirport sseFlightAirport `json:"departure_flight_airport"`
	ArrivalFlightAirport   sseFlightAirport `json:"arrival_flight_airport"`
	SeatsAvailable         int              `json:"seats_available"`
	Segments               []sseSegment     `json:"segments"`
}

type sseSegment struct {
	SegmentID              int              `json:"segment_id"`
	DepartureFlightAirport sseFlightAirport `json:"departure_flight_airport"`
	ArrivalFlightAirport   sseFlightAirport `json:"arrival_flight_airport"`
	DepartureTime          *string          `json:"departure_time,omitempty"`
	ArrivalTime            *string          `json:"arrival_time,omitempty"`
	DurationMinutes        *int             `json:"duration_minutes,omitempty"`
	FlightNumber           *string          `json:"flight_number,omitempty"`
	Operator               *sseOperator     `json:"operator,omitempty"`
}

type sseOperator struct {
	Name string `json:"name"`
	Code string `json:"code"`
	Logo string `json:"logo"`
}

type sseFlightAirport struct {
	Code     string `json:"code"`
	CityName string `json:"city_name"`
}

func mapOffers(src []flightsearch.EnrichedOffer) []sseOffer {
	offers := make([]sseOffer, len(src))
	for i := range src {
		offers[i] = sseOffer{
			OfferID:         src[i].OfferID,
			OutboundFlight:  mapFlight(src[i].OutboundFlight),
			CurrencyCode:    src[i].CurrencyCode,
			FareBand:        mapSSEFareBand(src[i].FareBand),
			Price:           src[i].Price,
			PassengerPrices: mapSSEPassengerPrices(src[i].PassengerPrices),
		}
		if src[i].InboundFlight != nil {
			inbound := mapFlight(*src[i].InboundFlight)
			offers[i].InboundFlight = &inbound
		}
	}
	return offers
}

func mapSSEPassengerPrices(src flightsession.PassengerPrices) ssePassengerPrices {
	return ssePassengerPrices{
		Adults:   nonNilSSEFloatList(src.Adults),
		Children: nonNilSSEFloatList(src.Children),
		Infants:  nonNilSSEFloatList(src.Infants),
	}
}

func nonNilSSEFloatList(values []float64) []float64 {
	if values == nil {
		return []float64{}
	}
	return values
}

func mapSSEFareBand(src flightsession.FareBand) sseFareBand {
	features := src.Features
	if features == nil {
		features = []string{}
	}
	return sseFareBand{
		Name:     src.Name,
		Features: features,
	}
}

func mapFlight(src flightsession.EnrichedFlight) sseFlight {
	segments := make([]sseSegment, len(src.Segments))
	for i := range src.Segments {
		segments[i] = sseSegment{
			SegmentID:              src.Segments[i].SegmentID,
			DepartureFlightAirport: mapSSEFlightAirport(src.Segments[i].DepartureFlightAirport),
			ArrivalFlightAirport:   mapSSEFlightAirport(src.Segments[i].ArrivalFlightAirport),
			DepartureTime:          formatLocalDateTime(src.Segments[i].DepartureTime),
			ArrivalTime:            formatLocalDateTime(src.Segments[i].ArrivalTime),
			DurationMinutes:        intPtr(src.Segments[i].DurationMinutes),
			FlightNumber:           stringPtr(src.Segments[i].FlightNumber),
			Operator:               mapSSEOperator(src.Segments[i].Operator),
		}
	}
	return sseFlight{
		DepartureFlightAirport: mapSSEFlightAirport(src.DepartureFlightAirport),
		ArrivalFlightAirport:   mapSSEFlightAirport(src.ArrivalFlightAirport),
		SeatsAvailable:         src.SeatsAvailable,
		Segments:               segments,
	}
}

func mapSSEFlightAirport(src flightsession.FlightAirport) sseFlightAirport {
	return sseFlightAirport{
		Code:     src.Code,
		CityName: src.CityName,
	}
}

func mapSSEOperator(src flightsession.EnrichedOperator) *sseOperator {
	if src.Name == "" && src.Code == "" && src.Logo == "" {
		return nil
	}
	return &sseOperator{
		Name: src.Name,
		Code: src.Code,
		Logo: src.Logo,
	}
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func formatLocalDateTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02T15:04:05")
	return &formatted
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
