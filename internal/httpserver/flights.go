package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/flights"
	"context"
	"errors"
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

	serviceResp, err := s.flightService.Search(ctx, serviceReq)
	if errors.Is(err, flights.ErrInvalidRequest) {
		return api.SearchFlights400JSONResponse{Message: err.Error()}, nil
	}
	if err != nil {
		return api.SearchFlights500JSONResponse{Message: err.Error()}, nil
	}

	return api.SearchFlights200JSONResponse{
		RoutingId: serviceResp.RoutingID,
		Offers:    mapOffers(serviceResp.Offers),
	}, nil
}

func mapOffers(src []flights.Offer) []api.Offer {
	offers := make([]api.Offer, len(src))
	for i := range src {
		offers[i] = api.Offer{
			OfferId:        src[i].OfferID,
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

func mapFlight(src flights.Flight) api.Flight {
	segments := make([]api.FlightSegment, len(src.Segments))
	for i := range src.Segments {
		segments[i] = api.FlightSegment{
			SegmentId:            int32(src.Segments[i].SegmentID),
			DepartureAirportCode: src.Segments[i].DepartureAirportCode,
			ArrivalAirportCode:   src.Segments[i].ArrivalAirportCode,
			DepartureTime:        src.Segments[i].DepartureTime,
			ArrivalTime:          src.Segments[i].ArrivalTime,
			DurationMinutes:      int32Ptr(src.Segments[i].DurationMinutes),
			FlightNumber:         stringPtr(src.Segments[i].FlightNumber),
			TravelClass:          stringPtr(src.Segments[i].TravelClass),
		}
	}
	return api.Flight{
		DepartureAirportCode: src.DepartureAirportCode,
		ArrivalAirportCode:   src.ArrivalAirportCode,
		SeatsAvailable:       int32(src.SeatsAvailable),
		Price:                src.Price,
		Segments:             segments,
	}
}

func int32Ptr(value int) *int32 {
	if value == 0 {
		return nil
	}
	converted := int32(value)
	return &converted
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
