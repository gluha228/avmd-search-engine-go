package search

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"fmt"
	"strings"
	"time"
)

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

func (s *Service) mapSearchOffers(req SearchRequest, outwardFlights, returnFlights []travelfusion.Flight) []Offer {
	filteredOutward := filterToDate(outwardFlights, req.DepartureDate)
	filteredReturns := []travelfusion.Flight(nil)
	if req.ReturnDate != nil {
		filteredReturns = filterToDate(returnFlights, *req.ReturnDate)
	}
	offers := applyFilters(buildOffers(filteredOutward, filteredReturns, req.ReturnDate != nil), req)
	sortOffers(offers)
	return offers
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

func flightMatchesFilters(flight session.Flight, req SearchRequest, outbound bool) bool {
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

func hasValidSegmentCount(segments []session.Segment, req SearchRequest) bool {
	return intInRange(len(segments), req.MinSegments, req.MaxSegments)
}

func hasValidDurations(segments []session.Segment, req SearchRequest) bool {
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

func hasValidLayovers(segments []session.Segment, req SearchRequest) bool {
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

func hasValidTimes(segments []session.Segment, req SearchRequest, outbound bool) bool {
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

func totalDurationMinutes(segments []session.Segment) (int, bool) {
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

	returnsByGroupID := flightsByGroupID(returnFlights)
	offers := make([]Offer, 0, len(outwardFlights))
	for _, outward := range outwardFlights {
		for _, inbound := range returnsByGroupID[outward.GroupID] {
			offers = append(offers, buildOffer(outward, &inbound))
		}
	}
	return offers
}

func flightsByGroupID(flights []travelfusion.Flight) map[string][]travelfusion.Flight {
	result := make(map[string][]travelfusion.Flight)
	for _, flight := range flights {
		groupID := strings.TrimSpace(flight.GroupID)
		if groupID == "" {
			continue
		}
		result[groupID] = append(result[groupID], flight)
	}
	return result
}

func buildOffer(outward travelfusion.Flight, inbound *travelfusion.Flight) Offer {
	outboundFlight := mapFlight(outward)
	price := outboundFlight.Price
	currency := outward.Currency

	var inboundFlight *session.Flight
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

func buildPassengerPrices(outward travelfusion.Flight, inbound *travelfusion.Flight) session.PassengerPrices {
	prices := mapPassengerPrices(outward.PassengerPrices)
	if inbound == nil {
		return prices
	}
	return addPassengerPrices(prices, mapPassengerPrices(inbound.PassengerPrices))
}

func mapPassengerPrices(src travelfusion.PassengerPrices) session.PassengerPrices {
	return session.PassengerPrices{
		Adults:   append([]float64(nil), src.Adults...),
		Children: append([]float64(nil), src.Children...),
		Infants:  append([]float64(nil), src.Infants...),
	}
}

func addPassengerPrices(left, right session.PassengerPrices) session.PassengerPrices {
	return session.PassengerPrices{
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

func fareBand(outward travelfusion.Flight, inbound *travelfusion.Flight) session.FareBand {
	name := minimalFareBandName(outward, inbound)
	if name == "" {
		name = "Economy"
	}
	return session.FareBand{Name: name, Features: []string{}}
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
