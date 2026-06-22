package flights

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid flight search request")
	iataCodePattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type TravelfusionClient interface {
	Search(ctx context.Context, req travelfusion.SearchRequest) (*travelfusion.SearchResult, error)
}

type SessionStore interface {
	Create(ctx context.Context, session FlightSearchSession) (string, error)
	Save(ctx context.Context, searchID string, session FlightSearchSession) error
}

type Service struct {
	tfClient     TravelfusionClient
	sessionStore SessionStore
	logger       *slog.Logger
	now          func() time.Time
}

func NewService(tfClient TravelfusionClient, logger *slog.Logger) *Service {
	return NewServiceWithSessionStore(tfClient, nil, logger)
}

func NewServiceWithSessionStore(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	logger *slog.Logger,
) *Service {
	return &Service{
		tfClient:     tfClient,
		sessionStore: sessionStore,
		logger:       logger,
		now:          time.Now,
	}
}

func (s *Service) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	searchID, err := s.CreateSession(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.SearchIntoSession(ctx, searchID, req, nil)
}

func (s *Service) CreateSession(ctx context.Context, req SearchRequest) (string, error) {
	if err := s.Validate(req); err != nil {
		return "", err
	}
	if s.sessionStore == nil {
		return "", nil
	}
	return s.sessionStore.Create(ctx, FlightSearchSession{Params: req})
}

func (s *Service) SearchIntoSession(
	ctx context.Context,
	searchID string,
	req SearchRequest,
	onOffers func([]Offer) error,
) (*SearchResponse, error) {
	if err := s.Validate(req); err != nil {
		return nil, err
	}

	tfResult, err := s.tfClient.Search(ctx, travelfusion.SearchRequest{
		DepartureAirportCode: req.DepartureAirportCode,
		ArrivalAirportCode:   req.ArrivalAirportCode,
		DepartureDate:        req.DepartureDate,
		ReturnDate:           req.ReturnDate,
		AdultCount:           req.AdultCount,
	})
	if err != nil {
		return nil, err
	}

	outwardFlights := filterToDate(tfResult.OutwardFlights, req.DepartureDate)
	returnFlights := []travelfusion.Flight(nil)
	if req.ReturnDate != nil {
		returnFlights = filterToDate(tfResult.ReturnFlights, *req.ReturnDate)
	}

	offers := buildOffers(outwardFlights, returnFlights, req.ReturnDate != nil)
	sort.Slice(offers, func(i, j int) bool {
		return offers[i].Price < offers[j].Price
	})
	if len(offers) > 0 && onOffers != nil {
		if err := onOffers(offers); err != nil {
			return nil, err
		}
	}

	if s.logger != nil {
		s.logger.Debug("flight search mapped", "routing_id", tfResult.RoutingID, "offers", len(offers))
	}

	if s.sessionStore != nil && searchID != "" {
		err = s.sessionStore.Save(ctx, searchID, FlightSearchSession{
			Params:      req,
			TFRoutingID: tfResult.RoutingID,
			TFOffers:    offers,
		})
		if err != nil {
			return nil, fmt.Errorf("update flight search session: %w", err)
		}
	}

	return &SearchResponse{
		SearchID:  searchID,
		RoutingID: tfResult.RoutingID,
		Offers:    offers,
	}, nil
}

func (s *Service) Validate(req SearchRequest) error {
	if !iataCodePattern.MatchString(req.DepartureAirportCode) {
		return fmt.Errorf("%w: departureAirportCode must be a 3-letter IATA code", ErrInvalidRequest)
	}
	if !iataCodePattern.MatchString(req.ArrivalAirportCode) {
		return fmt.Errorf("%w: arrivalAirportCode must be a 3-letter IATA code", ErrInvalidRequest)
	}
	if req.DepartureDate.IsZero() {
		return fmt.Errorf("%w: departureDate is required", ErrInvalidRequest)
	}
	if req.AdultCount < 1 {
		return fmt.Errorf("%w: adultCount must be at least 1", ErrInvalidRequest)
	}
	if compareCalendarDate(req.DepartureDate, s.now()) < 0 {
		return fmt.Errorf("%w: departureDate cannot be in the past", ErrInvalidRequest)
	}
	if req.ReturnDate != nil {
		if compareCalendarDate(*req.ReturnDate, s.now()) < 0 {
			return fmt.Errorf("%w: returnDate cannot be in the past", ErrInvalidRequest)
		}
		if compareCalendarDate(req.DepartureDate, *req.ReturnDate) > 0 {
			return fmt.Errorf("%w: departureDate cannot be after returnDate", ErrInvalidRequest)
		}
	}
	return nil
}

func filterToDate(flights []travelfusion.Flight, date time.Time) []travelfusion.Flight {
	filtered := make([]travelfusion.Flight, 0, len(flights))
	for _, flight := range flights {
		if flightMatchesDate(flight, date) {
			filtered = append(filtered, flight)
		}
	}
	return filtered
}

func flightMatchesDate(flight travelfusion.Flight, date time.Time) bool {
	if sameCalendarDate(flight.DepartureTime, date) {
		return true
	}
	for _, segment := range flight.Segments {
		if sameCalendarDate(segment.DepartureTime, date) {
			return true
		}
	}
	return false
}

func buildOffers(outwardFlights, returnFlights []travelfusion.Flight, roundTrip bool) []Offer {
	if !roundTrip {
		offers := make([]Offer, 0, len(outwardFlights))
		for _, outward := range outwardFlights {
			offers = append(offers, buildOffer(outward, nil))
		}
		return offers
	}

	count := len(outwardFlights)
	if len(returnFlights) < count {
		count = len(returnFlights)
	}
	offers := make([]Offer, 0, count)
	for i := 0; i < count; i++ {
		inbound := returnFlights[i]
		offers = append(offers, buildOffer(outwardFlights[i], &inbound))
	}
	if len(outwardFlights) > 0 && len(returnFlights) > 0 {
		cheapestOutward := cheapestFlight(outwardFlights)
		cheapestReturn := cheapestFlight(returnFlights)
		cheapest := buildOffer(cheapestOutward, &cheapestReturn)
		if !containsOffer(offers, cheapest.OfferID) {
			offers = append(offers, cheapest)
		}
	}
	return offers
}

func buildOffer(outward travelfusion.Flight, inbound *travelfusion.Flight) Offer {
	outboundFlight := mapFlight(outward)
	price := outboundFlight.Price
	currency := outward.Currency

	var inboundFlight *Flight
	if inbound != nil {
		mappedInbound := mapFlight(*inbound)
		inboundFlight = &mappedInbound
		price += mappedInbound.Price
		if currency == "" {
			currency = inbound.Currency
		}
	}

	return Offer{
		OfferID:        offerID(outward, inbound),
		OutboundFlight: outboundFlight,
		InboundFlight:  inboundFlight,
		CurrencyCode:   currency,
		Price:          price,
	}
}

func mapFlight(src travelfusion.Flight) Flight {
	segments := make([]Segment, 0, len(src.Segments))
	for i, segment := range src.Segments {
		segments = append(segments, Segment{
			SegmentID:            i + 1,
			DepartureAirportCode: segment.Origin,
			ArrivalAirportCode:   segment.Destination,
			DepartureTime:        timePtr(segment.DepartureTime),
			ArrivalTime:          timePtr(segment.ArrivalTime),
			DurationMinutes:      segment.DurationMinutes,
			FlightNumber:         segment.FlightNumber,
			TravelClass:          segment.TravelClass,
		})
	}
	return Flight{
		DepartureAirportCode: src.Origin,
		ArrivalAirportCode:   src.Destination,
		SeatsAvailable:       9,
		Price:                src.Price,
		Segments:             segments,
	}
}

func cheapestFlight(flights []travelfusion.Flight) travelfusion.Flight {
	cheapest := flights[0]
	for _, flight := range flights[1:] {
		if flight.Price < cheapest.Price {
			cheapest = flight
		}
	}
	return cheapest
}

func containsOffer(offers []Offer, offerID string) bool {
	for _, offer := range offers {
		if offer.OfferID == offerID {
			return true
		}
	}
	return false
}

func offerID(outward travelfusion.Flight, inbound *travelfusion.Flight) string {
	if inbound == nil {
		return outward.ID
	}
	return outward.ID + "_" + inbound.ID
}

func sameCalendarDate(a, b time.Time) bool {
	return compareCalendarDate(a, b) == 0
}

func compareCalendarDate(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	if ay != by {
		if ay < by {
			return -1
		}
		return 1
	}
	if am != bm {
		if am < bm {
			return -1
		}
		return 1
	}
	if ad != bd {
		if ad < bd {
			return -1
		}
		return 1
	}
	return 0
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
