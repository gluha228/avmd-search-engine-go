package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"fmt"
	"strings"
)

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

func (s *Service) loadFlightAirports(ctx context.Context, codes []string, locale string) (map[string]session.FlightAirport, error) {
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

func (s *Service) enrichOffer(offer Offer, airports map[string]session.FlightAirport) EnrichedOffer {
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

func (s *Service) enrichFlight(flight session.Flight, airports map[string]session.FlightAirport) session.EnrichedFlight {
	segments := make([]session.EnrichedSegment, len(flight.Segments))
	for i := range flight.Segments {
		segments[i] = session.EnrichedSegment{
			SegmentID:              flight.Segments[i].SegmentID,
			DepartureFlightAirport: flightAirportForCode(airports, flight.Segments[i].DepartureAirportCode),
			ArrivalFlightAirport:   flightAirportForCode(airports, flight.Segments[i].ArrivalAirportCode),
			DepartureTime:          flight.Segments[i].DepartureTime,
			ArrivalTime:            flight.Segments[i].ArrivalTime,
			DurationMinutes:        flight.Segments[i].DurationMinutes,
			FlightNumber:           flight.Segments[i].FlightNumber,
			TravelClass:            flight.Segments[i].TravelClass,
			Operator:               s.enrichOperator(flight.Segments[i].Operator),
		}
	}
	return session.EnrichedFlight{
		DepartureFlightAirport: flightAirportForCode(airports, flight.DepartureAirportCode),
		ArrivalFlightAirport:   flightAirportForCode(airports, flight.ArrivalAirportCode),
		SeatsAvailable:         flight.SeatsAvailable,
		Price:                  flight.Price,
		Segments:               segments,
	}
}

func (s *Service) enrichOperator(operator session.Operator) session.EnrichedOperator {
	return session.EnrichedOperator{
		Name: operator.Name,
		Code: operator.Code,
		Logo: s.operatorLogoURL(operator.Code),
	}
}

func (s *Service) operatorLogoURL(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	pattern := strings.TrimSpace(s.operatorLogoURLPattern)
	if pattern == "" {
		pattern = defaultOperatorLogoURLPattern
	}
	if strings.Contains(pattern, "%s") {
		return fmt.Sprintf(pattern, code)
	}
	return fmt.Sprintf("%sp%s.gif", strings.TrimRight(pattern, "/")+"/", code)
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

func collectFlightAirportCodes(flight session.Flight, seen map[string]struct{}, codes *[]string) {
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

func fallbackFlightAirportMap(codes []string) map[string]session.FlightAirport {
	result := make(map[string]session.FlightAirport, len(codes))
	for _, code := range codes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		result[code] = session.FlightAirport{Code: code, CityName: code}
	}
	return result
}

func flightAirportForCode(airports map[string]session.FlightAirport, code string) session.FlightAirport {
	code = strings.ToUpper(strings.TrimSpace(code))
	if airport, ok := airports[code]; ok {
		return airport
	}
	return session.FlightAirport{Code: code, CityName: code}
}

func normalizePassengerPrices(prices session.PassengerPrices) session.PassengerPrices {
	return session.PassengerPrices{
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

func normalizeFareBand(fareBand session.FareBand) session.FareBand {
	if strings.TrimSpace(fareBand.Name) == "" {
		fareBand.Name = "Economy"
	}
	if fareBand.Features == nil {
		fareBand.Features = []string{}
	}
	return fareBand
}

func (s *Service) convertOfferToDefaultCurrency(ctx context.Context, offer Offer) (Offer, error) {
	target := strings.ToUpper(strings.TrimSpace(s.defaultCurrency))
	source := strings.ToUpper(strings.TrimSpace(offer.CurrencyCode))
	if s.currency == nil || target == "" || source == "" || source == target {
		return offer, nil
	}

	originalTotal := offer.Price
	convertedTotal := 0.0
	convertedLegPrice := false
	if offer.OutboundFlight.Price != 0 {
		outbound, err := s.convertFlightToCurrency(ctx, offer.OutboundFlight, source, target)
		if err != nil {
			return Offer{}, fmt.Errorf("convert outbound offer price to %s: %w", target, err)
		}
		offer.OutboundFlight = outbound
		convertedTotal += outbound.Price
		convertedLegPrice = true
	}

	if offer.InboundFlight != nil && offer.InboundFlight.Price != 0 {
		inbound, err := s.convertFlightToCurrency(ctx, *offer.InboundFlight, source, target)
		if err != nil {
			return Offer{}, fmt.Errorf("convert inbound offer price to %s: %w", target, err)
		}
		offer.InboundFlight = &inbound
		convertedTotal += inbound.Price
		convertedLegPrice = true
	}

	if convertedLegPrice {
		offer.Price = convertedTotal
	} else {
		price, err := s.currency.Convert(ctx, originalTotal, source, target)
		if err != nil {
			return Offer{}, fmt.Errorf("convert offer price to %s: %w", target, err)
		}
		offer.Price = price
	}

	passengerPrices, err := s.convertPassengerPricesToCurrency(ctx, offer.PassengerPrices, source, target)
	if err != nil {
		return Offer{}, fmt.Errorf("convert passenger offer prices to %s: %w", target, err)
	}
	offer.PassengerPrices = passengerPrices
	offer.CurrencyCode = target
	return offer, nil
}

func (s *Service) convertFlightToCurrency(ctx context.Context, flight session.Flight, source, target string) (session.Flight, error) {
	converted, err := s.currency.Convert(ctx, flight.Price, source, target)
	if err != nil {
		return session.Flight{}, err
	}
	flight.Price = converted
	return flight, nil
}

func (s *Service) convertPassengerPricesToCurrency(ctx context.Context, prices session.PassengerPrices, source, target string) (session.PassengerPrices, error) {
	adults, err := s.convertAmountsToCurrency(ctx, prices.Adults, source, target)
	if err != nil {
		return session.PassengerPrices{}, err
	}
	children, err := s.convertAmountsToCurrency(ctx, prices.Children, source, target)
	if err != nil {
		return session.PassengerPrices{}, err
	}
	infants, err := s.convertAmountsToCurrency(ctx, prices.Infants, source, target)
	if err != nil {
		return session.PassengerPrices{}, err
	}
	return session.PassengerPrices{Adults: adults, Children: children, Infants: infants}, nil
}

func (s *Service) convertAmountsToCurrency(ctx context.Context, amounts []float64, source, target string) ([]float64, error) {
	if len(amounts) == 0 {
		return amounts, nil
	}
	converted := make([]float64, len(amounts))
	for i, amount := range amounts {
		value, err := s.currency.Convert(ctx, amount, source, target)
		if err != nil {
			return nil, err
		}
		converted[i] = value
	}
	return converted, nil
}
