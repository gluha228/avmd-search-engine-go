package flights

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid flight search request")
	ErrNotFound       = errors.New("flight resource not found")
	iataCodePattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type TravelfusionClient interface {
	Search(ctx context.Context, req travelfusion.SearchRequest) (*travelfusion.SearchResult, error)
	SearchStream(ctx context.Context, req travelfusion.SearchRequest) <-chan travelfusion.SearchUpdate
	ProcessDetails(ctx context.Context, req travelfusion.ProcessDetailsRequest) (*travelfusion.ProcessDetailsResult, error)
	ProcessTerms(ctx context.Context, req travelfusion.ProcessTermsRequest) (*travelfusion.ProcessTermsResult, error)
}

type SessionStore interface {
	Create(ctx context.Context, session FlightSearchSession) (string, error)
	Save(ctx context.Context, searchID string, session FlightSearchSession) error
	Get(ctx context.Context, searchID string) (*FlightSearchSession, error)
}

type CalendarCache interface {
	CacheFlights(ctx context.Context, departure, arrival string, flights []travelfusion.Flight) error
}

type CurrencyConverter interface {
	Convert(ctx context.Context, amount float64, from, to string) (float64, error)
}

type FlightAirportLookup interface {
	FlightAirportsByIATACodes(ctx context.Context, codes []string, locale string) (map[string]FlightAirport, error)
}

type Service struct {
	tfClient        TravelfusionClient
	sessionStore    SessionStore
	calendar        CalendarCache
	currency        CurrencyConverter
	airportLookup   FlightAirportLookup
	defaultCurrency string
	logger          *slog.Logger
	now             func() time.Time
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
	return NewServiceWithBookingDependencies(tfClient, sessionStore, calendarCache, nil, "", logger)
}

func NewServiceWithBookingDependencies(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	calendarCache CalendarCache,
	currencyConverter CurrencyConverter,
	defaultCurrency string,
	logger *slog.Logger,
) *Service {
	return NewServiceWithAirportLookup(tfClient, sessionStore, calendarCache, currencyConverter, nil, defaultCurrency, logger)
}

func NewServiceWithAirportLookup(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	calendarCache CalendarCache,
	currencyConverter CurrencyConverter,
	airportLookup FlightAirportLookup,
	defaultCurrency string,
	logger *slog.Logger,
) *Service {
	return &Service{
		tfClient:        tfClient,
		sessionStore:    sessionStore,
		calendar:        calendarCache,
		currency:        currencyConverter,
		airportLookup:   airportLookup,
		defaultCurrency: strings.ToUpper(strings.TrimSpace(defaultCurrency)),
		logger:          logger,
		now:             time.Now,
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
	var response SearchResponse
	seenUpdate := false
	for update := range s.SearchIntoSessionStream(ctx, searchID, req) {
		if update.Err != nil {
			return nil, update.Err
		}
		seenUpdate = true
		response.SearchID = update.SearchID
		response.RoutingID = update.RoutingID
		response.Offers = update.Offers
		if len(update.Offers) > 0 && onOffers != nil {
			if err := onOffers(update.Offers); err != nil {
				return nil, err
			}
		}
	}
	if !seenUpdate {
		response.SearchID = searchID
	}
	return &response, nil
}

func (s *Service) SearchIntoSessionStream(
	ctx context.Context,
	searchID string,
	req SearchRequest,
) <-chan SearchOffersUpdate {
	updates := make(chan SearchOffersUpdate)
	go func() {
		defer close(updates)
		s.searchIntoSessionStream(ctx, searchID, req, updates)
	}()
	return updates
}

func (s *Service) searchIntoSessionStream(
	ctx context.Context,
	searchID string,
	req SearchRequest,
	updates chan<- SearchOffersUpdate,
) {
	if err := s.Validate(req); err != nil {
		sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, Err: err})
		return
	}

	tfReq := travelfusion.SearchRequest{
		DepartureAirportCode: req.DepartureAirportCode,
		ArrivalAirportCode:   req.ArrivalAirportCode,
		DepartureDate:        req.DepartureDate,
		ReturnDate:           req.ReturnDate,
		AdultCount:           req.AdultCount,
		ChildCount:           req.ChildCount,
		InfantCount:          req.InfantCount,
	}

	var routingID string
	var outwardFlights []travelfusion.Flight
	var returnFlights []travelfusion.Flight
	var offers []Offer
	for tfUpdate := range s.tfClient.SearchStream(ctx, tfReq) {
		if tfUpdate.Err != nil {
			sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, RoutingID: routingID, Offers: offers, Err: tfUpdate.Err})
			return
		}
		if strings.TrimSpace(tfUpdate.RoutingID) != "" {
			routingID = tfUpdate.RoutingID
		}
		if s.calendar != nil {
			if len(tfUpdate.OutwardFlights) > 0 {
				if err := s.calendar.CacheFlights(ctx, req.DepartureAirportCode, req.ArrivalAirportCode, tfUpdate.OutwardFlights); err != nil && s.logger != nil {
					s.logger.Warn("failed to cache outbound calendar prices", "error", err)
				}
			}
			if req.ReturnDate != nil && len(tfUpdate.ReturnFlights) > 0 {
				if err := s.calendar.CacheFlights(ctx, req.ArrivalAirportCode, req.DepartureAirportCode, tfUpdate.ReturnFlights); err != nil && s.logger != nil {
					s.logger.Warn("failed to cache inbound calendar prices", "error", err)
				}
			}
		}
		outwardFlights = mergeTravelfusionFlights(outwardFlights, tfUpdate.OutwardFlights)
		returnFlights = mergeTravelfusionFlights(returnFlights, tfUpdate.ReturnFlights)

		offers = s.mapSearchOffers(req, outwardFlights, returnFlights)
		if s.logger != nil {
			s.logger.Debug("flight search mapped", "routing_id", routingID, "offers", len(offers))
		}

		if err := s.saveSearchSession(ctx, searchID, req, routingID, offers); err != nil {
			sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{SearchID: searchID, RoutingID: routingID, Offers: offers, Err: err})
			return
		}

		if !sendFlightSearchUpdate(ctx, updates, SearchOffersUpdate{
			SearchID:  searchID,
			RoutingID: routingID,
			Offers:    offers,
		}) {
			return
		}
	}
}

func (s *Service) mapSearchOffers(req SearchRequest, outwardFlights, returnFlights []travelfusion.Flight) []Offer {
	filteredOutward := filterToDate(outwardFlights, req.DepartureDate)
	filteredReturns := []travelfusion.Flight(nil)
	if req.ReturnDate != nil {
		filteredReturns = filterToDate(returnFlights, *req.ReturnDate)
	}
	offers := applyFilters(buildOffers(filteredOutward, filteredReturns, req.ReturnDate != nil), req)
	sort.Slice(offers, func(i, j int) bool {
		return offers[i].Price < offers[j].Price
	})
	return offers
}

func mergeTravelfusionFlights(current, next []travelfusion.Flight) []travelfusion.Flight {
	if len(next) == 0 {
		return current
	}
	byKey := make(map[string]int, len(current)+len(next))
	for i := range current {
		byKey[travelfusionFlightKey(current[i])] = i
	}
	for _, flight := range next {
		key := travelfusionFlightKey(flight)
		if existingIndex, ok := byKey[key]; ok {
			current[existingIndex] = flight
			continue
		}
		byKey[key] = len(current)
		current = append(current, flight)
	}
	return current
}

func travelfusionFlightKey(flight travelfusion.Flight) string {
	if strings.TrimSpace(flight.ID) != "" {
		return strings.TrimSpace(flight.ID)
	}
	return strings.Join([]string{
		flight.Origin,
		flight.Destination,
		flight.DepartureTime.Format(time.RFC3339Nano),
		flight.ArrivalTime.Format(time.RFC3339Nano),
		fmt.Sprintf("%.2f", flight.Price),
		flight.Currency,
	}, "|")
}

func (s *Service) saveSearchSession(ctx context.Context, searchID string, req SearchRequest, routingID string, offers []Offer) error {
	if s.sessionStore == nil || searchID == "" {
		return nil
	}
	err := s.sessionStore.Save(ctx, searchID, FlightSearchSession{
		Params:      req,
		TFRoutingID: routingID,
		TFOffers:    offers,
	})
	if err != nil {
		return fmt.Errorf("update flight search session: %w", err)
	}
	return nil
}

func sendFlightSearchUpdate(ctx context.Context, updates chan<- SearchOffersUpdate, update SearchOffersUpdate) bool {
	select {
	case <-ctx.Done():
		return false
	case updates <- update:
		return true
	}
}

func (s *Service) EnrichOffers(ctx context.Context, offers []Offer, locale string) ([]EnrichedOffer, error) {
	airportMap, err := s.loadFlightAirports(ctx, collectOfferAirportCodes(offers), locale)
	if err != nil {
		return nil, err
	}
	result := make([]EnrichedOffer, len(offers))
	for i := range offers {
		result[i] = s.enrichOffer(offers[i], airportMap)
	}
	return result, nil
}

func (s *Service) EnrichSearchOffers(ctx context.Context, offers []Offer, locale string) ([]EnrichedOffer, error) {
	converted := make([]Offer, len(offers))
	for i := range offers {
		offer, err := s.convertOfferToDefaultCurrency(ctx, offers[i])
		if err != nil {
			return nil, err
		}
		converted[i] = offer
	}
	return s.EnrichOffers(ctx, converted, locale)
}

func (s *Service) EnrichOffer(ctx context.Context, offer Offer, locale string) (EnrichedOffer, error) {
	enriched, err := s.EnrichOffers(ctx, []Offer{offer}, locale)
	if err != nil {
		return EnrichedOffer{}, err
	}
	if len(enriched) == 0 {
		return EnrichedOffer{}, nil
	}
	return enriched[0], nil
}

func (s *Service) loadFlightAirports(ctx context.Context, codes []string, locale string) (map[string]FlightAirport, error) {
	result := fallbackFlightAirportMap(codes)
	if s.airportLookup == nil || len(codes) == 0 {
		return result, nil
	}
	airports, err := s.airportLookup.FlightAirportsByIATACodes(ctx, codes, locale)
	if err != nil {
		return nil, err
	}
	for code, airport := range airports {
		normalizedCode := strings.ToUpper(strings.TrimSpace(code))
		if normalizedCode == "" {
			continue
		}
		if strings.TrimSpace(airport.Code) == "" {
			airport.Code = normalizedCode
		}
		result[normalizedCode] = airport
	}
	return result, nil
}

func (s *Service) enrichOffer(offer Offer, airports map[string]FlightAirport) EnrichedOffer {
	result := EnrichedOffer{
		OfferID:         offer.OfferID,
		OutboundFlight:  s.enrichFlight(offer.OutboundFlight, airports),
		CurrencyCode:    offer.CurrencyCode,
		FareBand:        normalizeFareBand(offer.FareBand),
		Price:           offer.Price,
		PassengerPrices: normalizePassengerPrices(offer.PassengerPrices),
	}
	if offer.InboundFlight != nil {
		inbound := s.enrichFlight(*offer.InboundFlight, airports)
		result.InboundFlight = &inbound
	}
	return result
}

func (s *Service) enrichFlight(flight Flight, airports map[string]FlightAirport) EnrichedFlight {
	segments := make([]EnrichedSegment, len(flight.Segments))
	for i := range flight.Segments {
		segments[i] = EnrichedSegment{
			SegmentID:              flight.Segments[i].SegmentID,
			DepartureFlightAirport: flightAirportForCode(airports, flight.Segments[i].DepartureAirportCode),
			ArrivalFlightAirport:   flightAirportForCode(airports, flight.Segments[i].ArrivalAirportCode),
			DepartureTime:          flight.Segments[i].DepartureTime,
			ArrivalTime:            flight.Segments[i].ArrivalTime,
			DurationMinutes:        flight.Segments[i].DurationMinutes,
			FlightNumber:           flight.Segments[i].FlightNumber,
			TravelClass:            flight.Segments[i].TravelClass,
		}
	}
	return EnrichedFlight{
		DepartureFlightAirport: flightAirportForCode(airports, flight.DepartureAirportCode),
		ArrivalFlightAirport:   flightAirportForCode(airports, flight.ArrivalAirportCode),
		SeatsAvailable:         flight.SeatsAvailable,
		Price:                  flight.Price,
		Segments:               segments,
	}
}

func collectOfferAirportCodes(offers []Offer) []string {
	seen := map[string]struct{}{}
	var codes []string
	for _, offer := range offers {
		collectFlightAirportCodes(offer.OutboundFlight, seen, &codes)
		if offer.InboundFlight != nil {
			collectFlightAirportCodes(*offer.InboundFlight, seen, &codes)
		}
	}
	return codes
}

func collectFlightAirportCodes(flight Flight, seen map[string]struct{}, codes *[]string) {
	addAirportCode(flight.DepartureAirportCode, seen, codes)
	addAirportCode(flight.ArrivalAirportCode, seen, codes)
	for _, segment := range flight.Segments {
		addAirportCode(segment.DepartureAirportCode, seen, codes)
		addAirportCode(segment.ArrivalAirportCode, seen, codes)
	}
}

func addAirportCode(code string, seen map[string]struct{}, codes *[]string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return
	}
	if _, ok := seen[code]; ok {
		return
	}
	seen[code] = struct{}{}
	*codes = append(*codes, code)
}

func fallbackFlightAirportMap(codes []string) map[string]FlightAirport {
	result := make(map[string]FlightAirport, len(codes))
	for _, code := range codes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		result[code] = FlightAirport{Code: code, CityName: code}
	}
	return result
}

func flightAirportForCode(airports map[string]FlightAirport, code string) FlightAirport {
	code = strings.ToUpper(strings.TrimSpace(code))
	if airport, ok := airports[code]; ok {
		return airport
	}
	return FlightAirport{Code: code, CityName: code}
}

func (s *Service) GetSelectedOffer(ctx context.Context, searchID, offerID string) (*SelectedOffer, error) {
	if s.sessionStore == nil {
		return nil, fmt.Errorf("%w: session store is not configured", ErrNotFound)
	}
	searchID = strings.TrimSpace(searchID)
	offerID = strings.TrimSpace(offerID)
	if searchID == "" {
		return nil, fmt.Errorf("%w: searchId is required", ErrInvalidRequest)
	}
	if offerID == "" {
		return nil, fmt.Errorf("%w: offerId is required", ErrInvalidRequest)
	}

	session, err := s.sessionStore.Get(ctx, searchID)
	if err != nil {
		return nil, fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, searchID)
	}
	if len(session.TFOffers) == 0 {
		return nil, fmt.Errorf("%w: no TravelFusion offers in session: %s", ErrNotFound, searchID)
	}

	offer, ok := findOffer(session.TFOffers, offerID)
	if !ok {
		return nil, fmt.Errorf("%w: offer with ID %s not found in TravelFusion session", ErrNotFound, offerID)
	}
	ids, ok := parseTFOfferID(offerID)
	if !ok {
		return nil, fmt.Errorf("%w: cannot parse TravelFusion outward/return ids from offerId=%s", ErrInvalidRequest, offerID)
	}
	if strings.TrimSpace(session.TFRoutingID) == "" {
		return nil, fmt.Errorf("TravelFusion routing id is missing for ancillary enrichment (offerId=%s)", offerID)
	}

	details, err := s.tfClient.ProcessDetails(ctx, travelfusion.ProcessDetailsRequest{
		RoutingID: session.TFRoutingID,
		OutwardID: ids.outwardID,
		ReturnID:  ids.returnID,
	})
	if err != nil {
		return nil, fmt.Errorf("TravelFusion ProcessDetails failed during ancillary enrichment (offerId=%s): %w", offerID, err)
	}
	if details == nil {
		return nil, fmt.Errorf("TravelFusion ProcessDetails returned empty response (offerId=%s)", offerID)
	}

	required := normalizeRequiredParameters(details.RequiredParameters)
	session.SelectedOfferID = offerID
	session.TFRequiredParameters = required
	s.cacheSeatMap(ctx, session, offerID, offer, required)
	if err := s.sessionStore.Save(ctx, searchID, *session); err != nil {
		return nil, fmt.Errorf("update selected offer session: %w", err)
	}

	offer, err = s.convertOfferToDefaultCurrency(ctx, offer)
	if err != nil {
		return nil, err
	}
	return &SelectedOffer{
		Offer:            offer,
		SearchParams:     session.Params,
		AdditionalFields: s.mapAdditionalFields(ctx, required),
	}, nil
}

func (s *Service) GetSeatMap(ctx context.Context, searchID, offerID string) ([]SegmentSeatMap, error) {
	if s.sessionStore == nil {
		return nil, fmt.Errorf("%w: session store is not configured", ErrNotFound)
	}
	searchID = strings.TrimSpace(searchID)
	offerID = strings.TrimSpace(offerID)
	if searchID == "" {
		return nil, fmt.Errorf("%w: searchId is required", ErrInvalidRequest)
	}
	if offerID == "" {
		return nil, fmt.Errorf("%w: offerId is required", ErrInvalidRequest)
	}

	session, err := s.sessionStore.Get(ctx, searchID)
	if err != nil {
		return nil, fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, searchID)
	}
	if session.TFSeatMapByOfferID == nil {
		return nil, fmt.Errorf("%w: TravelFusion seat map for offer %s not found in session %s", ErrNotFound, offerID, searchID)
	}
	seatMap, ok := session.TFSeatMapByOfferID[offerID]
	if !ok {
		return nil, fmt.Errorf("%w: TravelFusion seat map for offer %s not found in session %s", ErrNotFound, offerID, searchID)
	}
	return seatMap, nil
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
		OfferID:         offerID(outward, inbound),
		OutboundFlight:  outboundFlight,
		InboundFlight:   inboundFlight,
		CurrencyCode:    currency,
		FareBand:        fareBand(outward, inbound),
		Price:           price,
		PassengerPrices: buildPassengerPrices(outward, inbound),
	}
}

func buildPassengerPrices(outward travelfusion.Flight, inbound *travelfusion.Flight) PassengerPrices {
	prices := mapPassengerPrices(outward.PassengerPrices)
	if inbound == nil {
		return prices
	}
	return addPassengerPrices(prices, mapPassengerPrices(inbound.PassengerPrices))
}

func mapPassengerPrices(src travelfusion.PassengerPrices) PassengerPrices {
	return PassengerPrices{
		Adults:   append([]float64(nil), src.Adults...),
		Children: append([]float64(nil), src.Children...),
		Infants:  append([]float64(nil), src.Infants...),
	}
}

func addPassengerPrices(left, right PassengerPrices) PassengerPrices {
	return PassengerPrices{
		Adults:   addFloatLists(left.Adults, right.Adults),
		Children: addFloatLists(left.Children, right.Children),
		Infants:  addFloatLists(left.Infants, right.Infants),
	}
}

func addFloatLists(left, right []float64) []float64 {
	length := len(left)
	if len(right) > length {
		length = len(right)
	}
	if length == 0 {
		return nil
	}
	result := make([]float64, length)
	for i := 0; i < length; i++ {
		if i < len(left) {
			result[i] += left[i]
		}
		if i < len(right) {
			result[i] += right[i]
		}
	}
	return result
}

func normalizePassengerPrices(prices PassengerPrices) PassengerPrices {
	return PassengerPrices{
		Adults:   nonNilFloatList(prices.Adults),
		Children: nonNilFloatList(prices.Children),
		Infants:  nonNilFloatList(prices.Infants),
	}
}

func nonNilFloatList(values []float64) []float64 {
	if values == nil {
		return []float64{}
	}
	return values
}

func fareBand(outward travelfusion.Flight, inbound *travelfusion.Flight) FareBand {
	name := minimalFareBandName(outward, inbound)
	if name == "" {
		name = "Economy"
	}
	return FareBand{Name: name, Features: []string{}}
}

func normalizeFareBand(fareBand FareBand) FareBand {
	if strings.TrimSpace(fareBand.Name) == "" {
		fareBand.Name = "Economy"
	}
	if fareBand.Features == nil {
		fareBand.Features = []string{}
	}
	return fareBand
}

func minimalFareBandName(outward travelfusion.Flight, inbound *travelfusion.Flight) string {
	minClass := minimalKnownFlightClass(outward)
	if inbound != nil {
		minClass = lowerFlightClass(minClass, minimalKnownFlightClass(*inbound))
	}
	return flightClassName(minClass)
}

type flightClassRank int

const (
	unknownFlightClass      flightClassRank = -1
	economyWithRestrictions flightClassRank = iota
	economyWithoutRestrictions
	economyPremium
	business
	first
)

func minimalKnownFlightClass(flight travelfusion.Flight) flightClassRank {
	minClass := unknownFlightClass
	for _, segment := range flight.Segments {
		minClass = lowerFlightClass(minClass, parseFlightClass(segment.TravelClass))
	}
	return minClass
}

func lowerFlightClass(current, next flightClassRank) flightClassRank {
	if next == unknownFlightClass {
		return current
	}
	if current == unknownFlightClass || next < current {
		return next
	}
	return current
}

func parseFlightClass(value string) flightClassRank {
	switch strings.TrimSpace(value) {
	case "Economy", "Economy With Restrictions":
		return economyWithRestrictions
	case "Economy Without Restrictions":
		return economyWithoutRestrictions
	case "Economy Premium":
		return economyPremium
	case "Business":
		return business
	case "First":
		return first
	default:
		return unknownFlightClass
	}
}

func flightClassName(rank flightClassRank) string {
	switch rank {
	case economyWithRestrictions:
		return "Economy With Restrictions"
	case economyWithoutRestrictions:
		return "Economy Without Restrictions"
	case economyPremium:
		return "Economy Premium"
	case business:
		return "Business"
	case first:
		return "First"
	default:
		return ""
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
		return tfOfferIDPrefix + outward.ID
	}
	return tfOfferIDPrefix + outward.ID + "-" + inbound.ID
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
