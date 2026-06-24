package httpserver

import (
	api "avmd-search-engine-go/api/gen"
	flightbooking "avmd-search-engine-go/internal/flights/booking"
	flightsession "avmd-search-engine-go/internal/flights/session"
	"context"
	"errors"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s *HttpServer) SubmitBookingContactDetails(
	ctx context.Context,
	request api.SubmitBookingContactDetailsRequestObject,
) (api.SubmitBookingContactDetailsResponseObject, error) {
	if request.Body == nil {
		return api.SubmitBookingContactDetails400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: "request body is required"}}, nil
	}
	if err := s.validator.Struct(request.Body); err != nil {
		return api.SubmitBookingContactDetails400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	err := s.bookingService.SaveContactDetails(ctx, string(request.SearchId), mapBookingContactDetails(*request.Body))
	if errors.Is(err, flightbooking.ErrInvalidRequest) {
		return api.SubmitBookingContactDetails400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flightbooking.ErrNotFound) {
		return api.SubmitBookingContactDetails404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.SubmitBookingContactDetails500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.SubmitBookingContactDetails204Response{}, nil
}

func (s *HttpServer) SubmitPassengerData(
	ctx context.Context,
	request api.SubmitPassengerDataRequestObject,
) (api.SubmitPassengerDataResponseObject, error) {
	if request.Body == nil {
		return api.SubmitPassengerData400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: "request body is required"}}, nil
	}
	if err := s.validator.Struct(request.Body); err != nil {
		return api.SubmitPassengerData400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	serviceReq := mapPassengerDataRequest(*request.Body)
	response, err := s.bookingService.ProcessPassengerData(ctx, serviceReq)
	if errors.Is(err, flightbooking.ErrInvalidRequest) {
		return api.SubmitPassengerData400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flightbooking.ErrNotFound) {
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
	seatMap, err := s.bookingService.GetSeatMap(ctx, request.Params.SearchId, request.Params.OfferId)
	if errors.Is(err, flightbooking.ErrInvalidRequest) {
		return api.GetSeatMap400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flightbooking.ErrNotFound) {
		return api.GetSeatMap404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.GetSeatMap500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetSeatMap200JSONResponse(mapAPISegmentSeatMaps(seatMap)), nil
}

func mapPassengerDataRequest(src api.PassengerDataRequest) flightbooking.PassengerDataRequest {
	passengers := make([]flightsession.Passenger, len(src.Passengers))
	for i := range src.Passengers {
		passengers[i] = flightsession.Passenger{
			Title:                  string(src.Passengers[i].Title),
			FirstName:              src.Passengers[i].FirstName,
			LastName:               src.Passengers[i].LastName,
			DateOfBirth:            src.Passengers[i].DateOfBirth.Time,
			CitizenshipCountryCode: src.Passengers[i].CitizenshipCountryCode,
			SupplierParameters:     mapSupplierParameters(src.Passengers[i].SupplierParameters),
		}
	}
	return flightbooking.PassengerDataRequest{
		SearchID:   src.SearchId,
		OfferID:    src.OfferId,
		Passengers: passengers,
		ContactData: flightsession.ContactData{
			Email: string(src.ContactData.Email),
			Phone: flightsession.Phone{
				InternationalCode: src.ContactData.Phone.InternationalCode,
				Number:            src.ContactData.Phone.Number,
			},
		},
		SupplierParameters: mapSupplierParameters(src.SupplierParameters),
	}
}

func mapBookingContactDetails(src api.BookingContactDetails) flightsession.ContactData {
	return flightsession.ContactData{
		Email: string(src.Email),
		Phone: flightsession.Phone{
			InternationalCode: src.Phone.InternationalCode,
			Number:            src.Phone.Number,
		},
	}
}

func mapSupplierParameters(src *[]api.CustomSupplierParameter) []flightsession.SupplierParameter {
	if src == nil {
		return nil
	}
	result := make([]flightsession.SupplierParameter, len(*src))
	for i := range *src {
		result[i] = flightsession.SupplierParameter{
			ParamName:  (*src)[i].ParamName,
			ParamValue: (*src)[i].ParamValue,
		}
	}
	return result
}

func mapPassengerDataResponse(src flightbooking.PassengerDataResponse) api.PassengerDataResponse {
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
	ctx = flightbooking.WithLocale(ctx, locale)
	selectedOffer, err := s.bookingService.GetSelectedOffer(ctx, request.Params.SearchId, request.Params.OfferId)
	if errors.Is(err, flightbooking.ErrInvalidRequest) {
		return api.GetSelectedOffer400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Message: err.Error()}}, nil
	}
	if errors.Is(err, flightbooking.ErrNotFound) {
		return api.GetSelectedOffer404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Message: err.Error()}}, nil
	}
	if err != nil {
		return api.GetSelectedOffer500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	enrichedOffer, err := s.bookingService.EnrichOffer(ctx, selectedOffer.Offer, locale)
	if err != nil {
		return api.GetSelectedOffer500JSONResponse{InternalErrorJSONResponse: api.InternalErrorJSONResponse{Message: err.Error()}}, nil
	}
	return api.GetSelectedOffer200JSONResponse(mapSelectedOffer(*selectedOffer, enrichedOffer)), nil
}

func mapSelectedOffer(src flightbooking.SelectedOffer, offer flightbooking.EnrichedOffer) api.SelectedOffer {
	result := api.SelectedOffer{
		Offer:            mapAPIOffer(offer),
		SearchParams:     mapAPIFlightSearchParams(src.SearchParams),
		AdditionalFields: mapAPIAdditionalFields(src.AdditionalFields),
	}
	if src.ContactDetails != nil {
		contactDetails := mapAPIContactDetails(*src.ContactDetails)
		result.ContactDetails = &contactDetails
	}
	return result
}

func mapAPIContactDetails(src flightsession.ContactData) api.BookingContactDetails {
	return api.BookingContactDetails{
		Email: openapi_types.Email(strings.TrimSpace(src.Email)),
		Phone: api.PassengerPhone{
			InternationalCode: src.Phone.InternationalCode,
			Number:            src.Phone.Number,
		},
	}
}

func mapAPIOffer(src flightsession.EnrichedOffer) api.Offer {
	offer := api.Offer{
		OfferId:         src.OfferID,
		OutboundFlight:  mapAPIFlight(src.OutboundFlight),
		CurrencyCode:    src.CurrencyCode,
		FareBand:        mapAPIFareBand(src.FareBand),
		Price:           src.Price,
		PassengerPrices: mapAPIPassengerPrices(src.PassengerPrices),
	}
	if src.InboundFlight != nil {
		inbound := mapAPIFlight(*src.InboundFlight)
		offer.InboundFlight = &inbound
	}
	return offer
}

func mapAPIPassengerPrices(src flightsession.PassengerPrices) api.PassengerPrices {
	return api.PassengerPrices{
		Adults:   nonNilAPIFloatList(src.Adults),
		Children: nonNilAPIFloatList(src.Children),
		Infants:  nonNilAPIFloatList(src.Infants),
	}
}

func nonNilAPIFloatList(values []float64) []float64 {
	if values == nil {
		return []float64{}
	}
	return values
}

func mapAPIFareBand(src flightsession.FareBand) api.FareBand {
	features := src.Features
	if features == nil {
		features = []string{}
	}
	return api.FareBand{
		Name:     src.Name,
		Features: features,
	}
}

func mapAPIFlight(src flightsession.EnrichedFlight) api.Flight {
	segments := make([]api.FlightSegment, len(src.Segments))
	for i := range src.Segments {
		segments[i] = api.FlightSegment{
			SegmentId:              int32(src.Segments[i].SegmentID),
			DepartureFlightAirport: mapAPIFlightAirport(src.Segments[i].DepartureFlightAirport),
			ArrivalFlightAirport:   mapAPIFlightAirport(src.Segments[i].ArrivalFlightAirport),
			DepartureTime:          formatLocalDateTime(src.Segments[i].DepartureTime),
			ArrivalTime:            formatLocalDateTime(src.Segments[i].ArrivalTime),
			DurationMinutes:        optionalInt32(src.Segments[i].DurationMinutes),
			FlightNumber:           stringPtr(src.Segments[i].FlightNumber),
			Operator:               mapAPIFlightOperator(src.Segments[i].Operator),
		}
	}
	return api.Flight{
		DepartureFlightAirport: mapAPIFlightAirport(src.DepartureFlightAirport),
		ArrivalFlightAirport:   mapAPIFlightAirport(src.ArrivalFlightAirport),
		SeatsAvailable:         int32(src.SeatsAvailable),
		Segments:               segments,
	}
}

func mapAPIFlightAirport(src flightsession.FlightAirport) api.FlightAirport {
	return api.FlightAirport{
		Code:     src.Code,
		CityName: src.CityName,
	}
}

func mapAPIFlightOperator(src flightsession.EnrichedOperator) *api.FlightOperator {
	if src.Name == "" && src.Code == "" && src.Logo == "" {
		return nil
	}
	return &api.FlightOperator{
		Name: src.Name,
		Code: src.Code,
		Logo: src.Logo,
	}
}

func mapAPIFlightSearchParams(src flightsession.SearchRequest) api.FlightSearchParams {
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

func mapAPIAdditionalFields(src []flightsession.AdditionalField) []api.AdditionalField {
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

func mapAPISegmentSeatMaps(src []flightsession.SegmentSeatMap) []api.SegmentSeatMap {
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
				Available:                  src[i].Seats[j].IsAvailable,
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

func optionalClock(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("15:04")
	return &formatted
}
