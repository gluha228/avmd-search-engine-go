package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/flights"
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

	serviceReq := flights.SearchRequest{
		DepartureAirportCode: request.Params.DepartureAirportCode,
		ArrivalAirportCode:   request.Params.ArrivalAirportCode,
		DepartureDate:        request.Params.DepartureDate.Time,
		AdultCount:           int(request.Params.AdultCount),
	}
	if request.Params.ReturnDate != nil {
		returnDate := request.Params.ReturnDate.Time
		serviceReq.ReturnDate = &returnDate
	}

	if err := s.flightService.Validate(serviceReq); errors.Is(err, flights.ErrInvalidRequest) {
		return api.SearchFlights400JSONResponse{Message: err.Error()}, nil
	} else if err != nil {
		return api.SearchFlights500JSONResponse{Message: err.Error()}, nil
	}

	return searchFlightsSSEResponse{
		ctx:     ctx,
		service: s.flightService,
		request: serviceReq,
	}, nil
}

type searchFlightsSSEResponse struct {
	ctx     context.Context
	service *flights.Service
	request flights.SearchRequest
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

	if err := writeSSE(w, flusher, "search_id", map[string]string{"search_id": searchID}); err != nil {
		return err
	}

	_, err = response.service.SearchIntoSession(response.ctx, searchID, response.request, func(offers []flights.Offer) error {
		return writeSSE(w, flusher, "offers", map[string][]sseOffer{"offers": mapOffers(offers)})
	})
	if err != nil {
		_ = writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
		return nil
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
	OfferID        string     `json:"offer_id"`
	OutboundFlight sseFlight  `json:"outbound_flight"`
	InboundFlight  *sseFlight `json:"inbound_flight,omitempty"`
	CurrencyCode   string     `json:"currency_code"`
	Price          float64    `json:"price"`
}

type sseFlight struct {
	DepartureAirportCode string       `json:"departure_airport_code"`
	ArrivalAirportCode   string       `json:"arrival_airport_code"`
	SeatsAvailable       int          `json:"seats_available"`
	Price                float64      `json:"price"`
	Segments             []sseSegment `json:"segments"`
}

type sseSegment struct {
	SegmentID            int        `json:"segment_id"`
	DepartureAirportCode string     `json:"departure_airport_code"`
	ArrivalAirportCode   string     `json:"arrival_airport_code"`
	DepartureTime        *time.Time `json:"departure_time,omitempty"`
	ArrivalTime          *time.Time `json:"arrival_time,omitempty"`
	DurationMinutes      *int       `json:"duration_minutes,omitempty"`
	FlightNumber         *string    `json:"flight_number,omitempty"`
	TravelClass          *string    `json:"travel_class,omitempty"`
}

func mapOffers(src []flights.Offer) []sseOffer {
	offers := make([]sseOffer, len(src))
	for i := range src {
		offers[i] = sseOffer{
			OfferID:        src[i].OfferID,
			OutboundFlight: mapFlight(src[i].OutboundFlight),
			CurrencyCode:   src[i].CurrencyCode,
			Price:          src[i].Price,
		}
		if src[i].InboundFlight != nil {
			inbound := mapFlight(*src[i].InboundFlight)
			offers[i].InboundFlight = &inbound
		}
	}
	return offers
}

func mapFlight(src flights.Flight) sseFlight {
	segments := make([]sseSegment, len(src.Segments))
	for i := range src.Segments {
		segments[i] = sseSegment{
			SegmentID:            src.Segments[i].SegmentID,
			DepartureAirportCode: src.Segments[i].DepartureAirportCode,
			ArrivalAirportCode:   src.Segments[i].ArrivalAirportCode,
			DepartureTime:        src.Segments[i].DepartureTime,
			ArrivalTime:          src.Segments[i].ArrivalTime,
			DurationMinutes:      intPtr(src.Segments[i].DurationMinutes),
			FlightNumber:         stringPtr(src.Segments[i].FlightNumber),
			TravelClass:          stringPtr(src.Segments[i].TravelClass),
		}
	}
	return sseFlight{
		DepartureAirportCode: src.DepartureAirportCode,
		ArrivalAirportCode:   src.ArrivalAirportCode,
		SeatsAvailable:       src.SeatsAvailable,
		Price:                src.Price,
		Segments:             segments,
	}
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
