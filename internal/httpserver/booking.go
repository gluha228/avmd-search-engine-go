package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/flights"
	"context"
	"errors"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *HttpServer) SubmitPassengerData(
	ctx context.Context,
	request api.SubmitPassengerDataRequestObject,
) (api.SubmitPassengerDataResponseObject, error) {
	if request.Body == nil {
		return api.SubmitPassengerData400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: "request body is required"}}, nil
	}
	serviceReq := mapPassengerDataRequest(*request.Body)
	response, err := s.flightService.ProcessPassengerData(ctx, serviceReq)
	if errors.Is(err, flights.ErrInvalidRequest) {
		return api.SubmitPassengerData400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flights.ErrNotFound) {
		return api.SubmitPassengerData404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.SubmitPassengerData500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.SubmitPassengerData200JSONResponse(mapPassengerDataResponse(*response)), nil
}

func (s *HttpServer) GetSeatMap(
	ctx context.Context,
	request api.GetSeatMapRequestObject,
) (api.GetSeatMapResponseObject, error) {
	seatMap, err := s.flightService.GetSeatMap(ctx, request.Params.SearchId, request.Params.OfferId)
	if errors.Is(err, flights.ErrInvalidRequest) {
		return api.GetSeatMap400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flights.ErrNotFound) {
		return api.GetSeatMap404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.GetSeatMap500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetSeatMap200JSONResponse(mapAPISegmentSeatMaps(seatMap)), nil
}

func mapPassengerDataRequest(src api.PassengerDataRequest) flights.PassengerDataRequest {
	passengers := make([]flights.Passenger, len(src.Passengers))
	for i := range src.Passengers {
		passengers[i] = flights.Passenger{
			Title:                  string(src.Passengers[i].Title),
			FirstName:              src.Passengers[i].FirstName,
			LastName:               src.Passengers[i].LastName,
			DateOfBirth:            src.Passengers[i].DateOfBirth.Time,
			CitizenshipCountryCode: src.Passengers[i].CitizenshipCountryCode,
			SupplierParameters:     mapSupplierParameters(src.Passengers[i].SupplierParameters),
		}
	}
	return flights.PassengerDataRequest{
		SearchID:   src.SearchId,
		OfferID:    src.OfferId,
		Passengers: passengers,
		ContactData: flights.ContactData{
			Email: string(src.ContactData.Email),
			Phone: flights.Phone{
				InternationalCode: src.ContactData.Phone.InternationalCode,
				Number:            src.ContactData.Phone.Number,
			},
		},
		SupplierParameters: mapSupplierParameters(src.SupplierParameters),
	}
}

func mapSupplierParameters(src *[]api.CustomSupplierParameter) []flights.SupplierParameter {
	if src == nil {
		return nil
	}
	result := make([]flights.SupplierParameter, len(*src))
	for i := range *src {
		result[i] = flights.SupplierParameter{
			ParamName:  (*src)[i].ParamName,
			ParamValue: (*src)[i].ParamValue,
		}
	}
	return result
}

func mapPassengerDataResponse(src flights.PassengerDataResponse) api.PassengerDataResponse {
	responses := make([]api.ProcessTermsSupplierResponse, len(src.SupplierResponses))
	for i := range src.SupplierResponses {
		responses[i] = api.ProcessTermsSupplierResponse{
			Name: stringPtr(src.SupplierResponses[i].Name),
			Type: stringPtr(src.SupplierResponses[i].Type),
			Data: stringPtr(src.SupplierResponses[i].Data),
		}
	}
	return api.PassengerDataResponse{
		RoutingId:                           stringPtr(src.RoutingID),
		TfBookingReference:                  stringPtr(src.TFBookingReference),
		FinalAmount:                         src.FinalAmount,
		FinalCurrency:                       stringPtr(src.FinalCurrency),
		SupplierVisualAuthorisationImageUrl: stringPtr(src.SupplierVisualAuthorisationImageURL),
		SupplierResponses:                   responses,
	}
}

func (s *HttpServer) GetSelectedOffer(
	ctx context.Context,
	request api.GetSelectedOfferRequestObject,
) (api.GetSelectedOfferResponseObject, error) {
	locale := localeFromContext(ctx)
	ctx = flights.WithLocale(ctx, locale)
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
	enrichedOffer, err := s.flightService.EnrichOffer(ctx, selectedOffer.Offer, locale)
	if err != nil {
		return api.GetSelectedOffer500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetSelectedOffer200JSONResponse(mapSelectedOffer(*selectedOffer, enrichedOffer)), nil
}

func mapSelectedOffer(src flights.SelectedOffer, offer flights.EnrichedOffer) api.SelectedOffer {
	return api.SelectedOffer{
		Offer:            mapAPIOffer(offer),
		SearchParams:     mapAPIFlightSearchParams(src.SearchParams),
		AdditionalFields: mapAPIAdditionalFields(src.AdditionalFields),
	}
}

func mapAPIOffer(src flights.EnrichedOffer) api.Offer {
	offer := api.Offer{
		OfferId:        src.OfferID,
		OutboundFlight: mapAPIFlight(src.OutboundFlight),
		CurrencyCode:   src.CurrencyCode,
		FareBand:       mapAPIFareBand(src.FareBand),
		Price:          src.Price,
	}
	if src.InboundFlight != nil {
		inbound := mapAPIFlight(*src.InboundFlight)
		offer.InboundFlight = &inbound
	}
	return offer
}

func mapAPIFareBand(src flights.FareBand) api.FareBand {
	features := src.Features
	if features == nil {
		features = []string{}
	}
	return api.FareBand{
		Name:     src.Name,
		Features: features,
	}
}

func mapAPIFlight(src flights.EnrichedFlight) api.Flight {
	segments := make([]api.FlightSegment, len(src.Segments))
	for i := range src.Segments {
		segments[i] = api.FlightSegment{
			SegmentId:              int32(src.Segments[i].SegmentID),
			DepartureFlightAirport: mapAPIFlightAirport(src.Segments[i].DepartureFlightAirport),
			ArrivalFlightAirport:   mapAPIFlightAirport(src.Segments[i].ArrivalFlightAirport),
			DepartureTime:          formatLocalDateTime(src.Segments[i].DepartureTime),
			ArrivalTime:            formatLocalDateTime(src.Segments[i].ArrivalTime),
			DurationMinutes:        int32Ptr(src.Segments[i].DurationMinutes),
			FlightNumber:           stringPtr(src.Segments[i].FlightNumber),
		}
	}
	return api.Flight{
		DepartureFlightAirport: mapAPIFlightAirport(src.DepartureFlightAirport),
		ArrivalFlightAirport:   mapAPIFlightAirport(src.ArrivalFlightAirport),
		SeatsAvailable:         int32(src.SeatsAvailable),
		Segments:               segments,
	}
}

func mapAPIFlightAirport(src flights.FlightAirport) api.FlightAirport {
	return api.FlightAirport{
		Code:     src.Code,
		CityName: src.CityName,
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

func mapAPISegmentSeatMaps(src []flights.SegmentSeatMap) []api.SegmentSeatMap {
	result := make([]api.SegmentSeatMap, len(src))
	for i := range src {
		seats := make([]api.SeatDetail, len(src[i].Seats))
		for j := range src[i].Seats {
			seats[j] = api.SeatDetail{
				Code:                       src[i].Seats[j].Code,
				Type:                       api.SeatType(src[i].Seats[j].Type),
				SeatDescription:            src[i].Seats[j].SeatDescription,
				Price:                      src[i].Seats[j].Price,
				CurrencyCode:               src[i].Seats[j].CurrencyCode,
				Row:                        int32(src[i].Seats[j].Row),
				Col:                        int32(src[i].Seats[j].Col),
				IsAvailable:                src[i].Seats[j].IsAvailable,
				PersonsWithReducedMobility: src[i].Seats[j].PersonsWithReducedMobility,
				NoInfantSeat:               src[i].Seats[j].NoInfantSeat,
			}
		}
		result[i] = api.SegmentSeatMap{
			SegmentId:    int32(src[i].SegmentID),
			Origin:       src[i].Origin,
			Destination:  src[i].Destination,
			FlightNumber: src[i].FlightNumber,
			Seats:        seats,
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
