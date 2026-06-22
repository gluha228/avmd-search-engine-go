package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/flights"
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *HttpServer) GetSelectedOffer(
	ctx context.Context,
	request api.GetSelectedOfferRequestObject,
) (api.GetSelectedOfferResponseObject, error) {
	ctx = flights.WithLocale(ctx, localeFromContext(ctx))
	selectedOffer, err := s.flightService.GetSelectedOffer(ctx, request.Params.SearchId, request.Params.OfferId)
	if errors.Is(err, flights.ErrInvalidRequest) {
		return api.GetSelectedOffer400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flights.ErrNotFound) {
		return api.GetSelectedOffer404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.GetSelectedOffer500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetSelectedOffer200JSONResponse(mapSelectedOffer(*selectedOffer)), nil
}

func mapSelectedOffer(src flights.SelectedOffer) api.SelectedOffer {
	return api.SelectedOffer{
		Offer:            mapAPIOffer(src.Offer),
		SearchParams:     mapAPIFlightSearchParams(src.SearchParams),
		AdditionalFields: mapAPIAdditionalFields(src.AdditionalFields),
	}
}

func mapAPIOffer(src flights.Offer) api.Offer {
	offer := api.Offer{
		OfferId:        src.OfferID,
		OutboundFlight: mapAPIFlight(src.OutboundFlight),
		CurrencyCode:   src.CurrencyCode,
		Price:          src.Price,
	}
	if src.InboundFlight != nil {
		inbound := mapAPIFlight(*src.InboundFlight)
		offer.InboundFlight = &inbound
	}
	return offer
}

func mapAPIFlight(src flights.Flight) api.Flight {
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

func mapAPIFlightSearchParams(src flights.SearchRequest) api.FlightSearchParams {
	params := api.FlightSearchParams{
		DepartureAirportCode:                src.DepartureAirportCode,
		ArrivalAirportCode:                  src.ArrivalAirportCode,
		DepartureDate:                       openapi_types.Date{Time: src.DepartureDate},
		AdultCount:                          int32(src.AdultCount),
		ChildCount:                          optionalInt32(src.ChildCount),
		InfantCount:                         optionalInt32(src.InfantCount),
		MinPrice:                            src.MinPrice,
		MaxPrice:                            src.MaxPrice,
		MinSegments:                         optionalIntPointer(src.MinSegments),
		MaxSegments:                         optionalIntPointer(src.MaxSegments),
		MinTotalDurationMinutes:             optionalIntPointer(src.MinTotalDurationMinutes),
		MaxTotalDurationMinutes:             optionalIntPointer(src.MaxTotalDurationMinutes),
		MinIndividualSegmentDurationMinutes: optionalIntPointer(src.MinIndividualSegmentDurationMinutes),
		MaxIndividualSegmentDurationMinutes: optionalIntPointer(src.MaxIndividualSegmentDurationMinutes),
		MinLayoverMinutes:                   optionalIntPointer(src.MinLayoverMinutes),
		MaxLayoverMinutes:                   optionalIntPointer(src.MaxLayoverMinutes),
		DepartureOutboundFrom:               optionalClock(src.DepartureOutboundFrom),
		DepartureOutboundTo:                 optionalClock(src.DepartureOutboundTo),
		ArrivalOutboundFrom:                 optionalClock(src.ArrivalOutboundFrom),
		ArrivalOutboundTo:                   optionalClock(src.ArrivalOutboundTo),
		DepartureInboundFrom:                optionalClock(src.DepartureInboundFrom),
		DepartureInboundTo:                  optionalClock(src.DepartureInboundTo),
		ArrivalInboundFrom:                  optionalClock(src.ArrivalInboundFrom),
		ArrivalInboundTo:                    optionalClock(src.ArrivalInboundTo),
	}
	if src.ReturnDate != nil {
		params.ReturnDate = &openapi_types.Date{Time: *src.ReturnDate}
	}
	return params
}

func mapAPIAdditionalFields(src []flights.AdditionalField) []api.AdditionalField {
	result := make([]api.AdditionalField, len(src))
	for i := range src {
		options := make([]api.AdditionalFieldOption, len(src[i].Options))
		for j := range src[i].Options {
			options[j] = api.AdditionalFieldOption{
				Value: stringPtr(src[i].Options[j].Value),
				Label: stringPtr(src[i].Options[j].Label),
			}
			if src[i].Options[j].Price != nil {
				options[j].Price = &api.AdditionalFieldOptionPrice{
					Amount:       src[i].Options[j].Price.Amount,
					CurrencyCode: src[i].Options[j].Price.CurrencyCode,
				}
			}
		}
		result[i] = api.AdditionalField{
			Code:         src[i].Code,
			Description:  stringPtr(src[i].Description),
			InputType:    src[i].InputType,
			Required:     src[i].Required,
			PerPassenger: src[i].PerPassenger,
			Options:      options,
		}
	}
	return result
}

func optionalInt32(value int) *int32 {
	if value == 0 {
		return nil
	}
	converted := int32(value)
	return &converted
}

func optionalIntPointer(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func optionalFloat64(value float64) *float64 {
	if value == 0 {
		return nil
	}
	return &value
}

func int32Ptr(value int) *int32 {
	if value == 0 {
		return nil
	}
	converted := int32(value)
	return &converted
}

func optionalClock(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("15:04")
	return &formatted
}
