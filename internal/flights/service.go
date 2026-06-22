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

type CalendarCache interface {
	CacheFlights(ctx context.Context, departure, arrival string, flights []travelfusion.Flight) error
}

type Service struct {
	tfClient     TravelfusionClient
	sessionStore SessionStore
	calendar     CalendarCache
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
	return NewServiceWithDependencies(tfClient, sessionStore, nil, logger)
}

func NewServiceWithDependencies(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	calendarCache CalendarCache,
	logger *slog.Logger,
) *Service {
	return &Service{
		tfClient:     tfClient,
		sessionStore: sessionStore,
		calendar:     calendarCache,
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
		ChildCount:           req.ChildCount,
		InfantCount:          req.InfantCount,
	})
	if err != nil {
		return nil, err
	}
	if s.calendar != nil {
		if err := s.calendar.CacheFlights(ctx, req.DepartureAirportCode, req.ArrivalAirportCode, tfResult.OutwardFlights); err != nil && s.logger != nil {
			s.logger.Warn("failed to cache outbound calendar prices", "error", err)
		}
		if req.ReturnDate != nil {
			if err := s.calendar.CacheFlights(ctx, req.ArrivalAirportCode, req.DepartureAirportCode, tfResult.ReturnFlights); err != nil && s.logger != nil {
				s.logger.Warn("failed to cache inbound calendar prices", "error", err)
			}
		}
	}

	outwardFlights := filterToDate(tfResult.OutwardFlights, req.DepartureDate)
	returnFlights := []travelfusion.Flight(nil)
	if req.ReturnDate != nil {
		returnFlights = filterToDate(tfResult.ReturnFlights, *req.ReturnDate)
	}

	offers := applyFilters(buildOffers(outwardFlights, returnFlights, req.ReturnDate != nil), req)
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
	if req.ChildCount < 0 {
		return fmt.Errorf("%w: childCount cannot be negative", ErrInvalidRequest)
	}
	if req.InfantCount < 0 {
		return fmt.Errorf("%w: infantCount cannot be negative", ErrInvalidRequest)
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
	if err := validateFilters(req); err != nil {
		return err
	}
	return nil
}

func validateFilters(req SearchRequest) error {
	if err := validateFloatRange("price", req.MinPrice, req.MaxPrice); err != nil {
		return err
	}
	if err := validateIntRange("segments", req.MinSegments, req.MaxSegments); err != nil {
		return err
	}
	if err := validateIntRange("total duration", req.MinTotalDurationMinutes, req.MaxTotalDurationMinutes); err != nil {
		return err
	}
	if err := validateIntRange("individual segment duration", req.MinIndividualSegmentDurationMinutes, req.MaxIndividualSegmentDurationMinutes); err != nil {
		return err
	}
	if err := validateIntRange("layover", req.MinLayoverMinutes, req.MaxLayoverMinutes); err != nil {
		return err
	}
	if err := validateTimeRange("outbound departure", req.DepartureOutboundFrom, req.DepartureOutboundTo); err != nil {
		return err
	}
	if err := validateTimeRange("outbound arrival", req.ArrivalOutboundFrom, req.ArrivalOutboundTo); err != nil {
		return err
	}
	if err := validateTimeRange("inbound departure", req.DepartureInboundFrom, req.DepartureInboundTo); err != nil {
		return err
	}
	return validateTimeRange("inbound arrival", req.ArrivalInboundFrom, req.ArrivalInboundTo)
}

func validateFloatRange(name string, min *float64, max *float64) error {
	if min != nil && *min < 0 {
		return fmt.Errorf("%w: minimum %s cannot be negative", ErrInvalidRequest, name)
	}
	if max != nil && *max < 0 {
		return fmt.Errorf("%w: maximum %s cannot be negative", ErrInvalidRequest, name)
	}
	if min != nil && max != nil && *min > *max {
		return fmt.Errorf("%w: minimum %s cannot be greater than maximum %s", ErrInvalidRequest, name, name)
	}
	return nil
}

func validateIntRange(name string, min *int, max *int) error {
	if min != nil && *min < 0 {
		return fmt.Errorf("%w: minimum %s cannot be negative", ErrInvalidRequest, name)
	}
	if max != nil && *max < 0 {
		return fmt.Errorf("%w: maximum %s cannot be negative", ErrInvalidRequest, name)
	}
	if min != nil && max != nil && *min > *max {
		return fmt.Errorf("%w: minimum %s cannot be greater than maximum %s", ErrInvalidRequest, name, name)
	}
	return nil
}

func validateTimeRange(name string, from *time.Time, to *time.Time) error {
	if from != nil && to != nil && compareClockTime(*from, *to) > 0 {
		return fmt.Errorf("%w: %s start time must be before or equal to end time", ErrInvalidRequest, name)
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

func applyFilters(offers []Offer, req SearchRequest) []Offer {
	filtered := make([]Offer, 0, len(offers))
	for _, offer := range offers {
		if !floatInRange(offer.Price, req.MinPrice, req.MaxPrice) {
			continue
		}
		if !flightMatchesFilters(offer.OutboundFlight, req, true) {
			continue
		}
		if offer.InboundFlight != nil && !flightMatchesFilters(*offer.InboundFlight, req, false) {
			continue
		}
		filtered = append(filtered, offer)
	}
	return filtered
}

func flightMatchesFilters(flight Flight, req SearchRequest, outbound bool) bool {
	segments := flight.Segments
	if len(segments) == 0 {
		return !hasSegmentFilters(req, outbound)
	}
	return hasValidSegmentCount(segments, req) &&
		hasValidDurations(segments, req) &&
		hasValidLayovers(segments, req) &&
		hasValidTimes(segments, req, outbound)
}

func hasSegmentFilters(req SearchRequest, outbound bool) bool {
	if req.MinSegments != nil || req.MaxSegments != nil ||
		req.MinTotalDurationMinutes != nil || req.MaxTotalDurationMinutes != nil ||
		req.MinIndividualSegmentDurationMinutes != nil || req.MaxIndividualSegmentDurationMinutes != nil ||
		req.MinLayoverMinutes != nil || req.MaxLayoverMinutes != nil {
		return true
	}
	if outbound {
		return req.DepartureOutboundFrom != nil || req.DepartureOutboundTo != nil ||
			req.ArrivalOutboundFrom != nil || req.ArrivalOutboundTo != nil
	}
	return req.DepartureInboundFrom != nil || req.DepartureInboundTo != nil ||
		req.ArrivalInboundFrom != nil || req.ArrivalInboundTo != nil
}

func hasValidSegmentCount(segments []Segment, req SearchRequest) bool {
	return intInRange(len(segments), req.MinSegments, req.MaxSegments)
}

func hasValidDurations(segments []Segment, req SearchRequest) bool {
	totalDuration, ok := totalDurationMinutes(segments)
	if !ok || !intInRange(totalDuration, req.MinTotalDurationMinutes, req.MaxTotalDurationMinutes) {
		return false
	}
	for _, segment := range segments {
		if !intInRange(segment.DurationMinutes, req.MinIndividualSegmentDurationMinutes, req.MaxIndividualSegmentDurationMinutes) {
			return false
		}
	}
	return true
}

func hasValidLayovers(segments []Segment, req SearchRequest) bool {
	for i := 0; i < len(segments)-1; i++ {
		if segments[i].ArrivalTime == nil || segments[i+1].DepartureTime == nil {
			return false
		}
		layover := int(segments[i+1].DepartureTime.Sub(*segments[i].ArrivalTime).Minutes())
		if !intInRange(layover, req.MinLayoverMinutes, req.MaxLayoverMinutes) {
			return false
		}
	}
	return true
}

func hasValidTimes(segments []Segment, req SearchRequest, outbound bool) bool {
	firstDeparture := segments[0].DepartureTime
	lastArrival := segments[len(segments)-1].ArrivalTime
	if firstDeparture == nil || lastArrival == nil {
		return false
	}
	if outbound {
		return timeInRange(*firstDeparture, req.DepartureOutboundFrom, req.DepartureOutboundTo) &&
			timeInRange(*lastArrival, req.ArrivalOutboundFrom, req.ArrivalOutboundTo)
	}
	return timeInRange(*firstDeparture, req.DepartureInboundFrom, req.DepartureInboundTo) &&
		timeInRange(*lastArrival, req.ArrivalInboundFrom, req.ArrivalInboundTo)
}

func totalDurationMinutes(segments []Segment) (int, bool) {
	if len(segments) == 0 || segments[0].DepartureTime == nil || segments[len(segments)-1].ArrivalTime == nil {
		return 0, false
	}
	return int(segments[len(segments)-1].ArrivalTime.Sub(*segments[0].DepartureTime).Minutes()), true
}

func floatInRange(value float64, min *float64, max *float64) bool {
	return (min == nil || value >= *min) && (max == nil || value <= *max)
}

func intInRange(value int, min *int, max *int) bool {
	return (min == nil || value >= *min) && (max == nil || value <= *max)
}

func timeInRange(value time.Time, from *time.Time, to *time.Time) bool {
	return (from == nil || compareClockTime(value, *from) >= 0) &&
		(to == nil || compareClockTime(value, *to) <= 0)
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

func compareClockTime(a, b time.Time) int {
	aMinutes := a.Hour()*60 + a.Minute()
	bMinutes := b.Hour()*60 + b.Minute()
	if aMinutes < bMinutes {
		return -1
	}
	if aMinutes > bMinutes {
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
