package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"regexp"
	"strconv"
	"strings"
)

const (
	SeatTypeStandard     = "STANDARD"
	SeatTypeExitRow      = "EXIT_ROW"
	SeatTypeWindow       = "WINDOW"
	SeatTypeAisle        = "AISLE"
	SeatTypeCentre       = "CENTRE"
	SeatTypeMiddle       = "MIDDLE"
	SeatTypeExtraLegroom = "EXTRA_LEGROOM"
)

var (
	seatEntryPattern = regexp.MustCompile(`([A-Z0-9]+)-(\d+)([A-Z]+)\(([^)]*)\)`)
	seatPricePattern = regexp.MustCompile(`^([\d.]+)([A-Z]+)$`)
	noSeatAttributes = map[string]struct{}{"CL": {}, "D": {}, "EX": {}, "GN": {}, "LA": {}, "ST": {}, "NS": {}}
)

func (s *Service) cacheSeatMap(ctx context.Context, searchSession *session.FlightSearchSession, offerID string, offer Offer, required []session.TFRequiredParameterSnapshot) {
	if searchSession.TFSeatMapByOfferID == nil {
		searchSession.TFSeatMapByOfferID = map[string][]session.SegmentSeatMap{}
	}

	rawSeatOptions, ok := extractSeatOptions(required)
	if !ok || !hasSeatOptions(rawSeatOptions) {
		delete(searchSession.TFSeatMapByOfferID, offerID)
		return
	}

	seatMap := parseSeatOptions(rawSeatOptions, collectOfferSegments(offer))
	s.normalizeSeatPrices(ctx, seatMap)
	searchSession.TFSeatMapByOfferID[offerID] = seatMap
}

func extractSeatOptions(required []session.TFRequiredParameterSnapshot) (string, bool) {
	for _, parameter := range required {
		if parameter.Parameter == "SEAT_OPTIONS" && strings.TrimSpace(parameter.DisplayText) != "" {
			return parameter.DisplayText, true
		}
	}
	return "", false
}

func collectOfferSegments(offer Offer) []session.Segment {
	segments := make([]session.Segment, 0, len(offer.OutboundFlight.Segments))
	segments = append(segments, offer.OutboundFlight.Segments...)
	if offer.InboundFlight != nil {
		segments = append(segments, offer.InboundFlight.Segments...)
	}
	return segments
}

func hasSeatOptions(rawSeatOptions string) bool {
	if strings.TrimSpace(rawSeatOptions) == "" {
		return false
	}
	colonIdx := strings.Index(rawSeatOptions, ":")
	if colonIdx < 0 || colonIdx == len(rawSeatOptions)-1 {
		return false
	}
	for _, token := range strings.Split(rawSeatOptions[colonIdx+1:], ";") {
		if strings.TrimSpace(token) != "" {
			return true
		}
	}
	return false
}

func parseSeatOptions(rawSeatOptions string, segments []session.Segment) []session.SegmentSeatMap {
	if strings.TrimSpace(rawSeatOptions) == "" || len(segments) == 0 {
		return []session.SegmentSeatMap{}
	}

	optionsPart := rawSeatOptions
	if colonIdx := strings.Index(rawSeatOptions, ":"); colonIdx >= 0 {
		optionsPart = rawSeatOptions[colonIdx+1:]
	}
	blocks := strings.Split(optionsPart, ";")
	result := make([]session.SegmentSeatMap, 0, len(segments))
	for i, segment := range segments {
		seats := []session.SeatDetail{}
		if i < len(blocks) && strings.TrimSpace(blocks[i]) != "" {
			seats = parseSeats(blocks[i])
		}
		result = append(result, session.SegmentSeatMap{
			SegmentID:    segment.SegmentID,
			Origin:       segment.DepartureAirportCode,
			Destination:  segment.ArrivalAirportCode,
			FlightNumber: segment.FlightNumber,
			Seats:        seats,
		})
	}
	return result
}

func parseSeats(block string) []session.SeatDetail {
	matches := seatEntryPattern.FindAllStringSubmatch(block, -1)
	seats := make([]session.SeatDetail, 0, len(matches))
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		row, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		colStr := match[3]
		if colStr == "" {
			continue
		}
		attrs, price, currency := parseSeatContent(match[4])
		isNoSeat := hasAnyAttr(attrs, noSeatAttributes)
		isUnavailable := hasAttr(attrs, "T") || isNoSeat
		seats = append(seats, session.SeatDetail{
			Code:                       match[2] + colStr,
			Type:                       deriveSeatType(attrs),
			SeatDescription:            buildSeatDescription(attrs),
			Price:                      price,
			CurrencyCode:               currency,
			Row:                        row,
			Col:                        int(colStr[0] - 'A'),
			IsAvailable:                !isUnavailable,
			PersonsWithReducedMobility: hasAttr(attrs, "H"),
			NoInfantSeat:               hasAttr(attrs, "1A") || hasAttr(attrs, "1I") || hasAttr(attrs, "IE"),
		})
	}
	return seats
}

func parseSeatContent(content string) (map[string]struct{}, *float64, *string) {
	atParts := strings.SplitN(content, "@", 3)
	attrPart := atParts[0]
	priceAndCurrency := ""
	if len(atParts) > 1 {
		priceAndCurrency = strings.TrimSpace(atParts[1])
	}

	attrs := map[string]struct{}{}
	for _, attr := range strings.Split(attrPart, "|") {
		attr = strings.ToUpper(strings.TrimSpace(attr))
		if attr != "" {
			attrs[attr] = struct{}{}
		}
	}

	if priceAndCurrency == "" {
		return attrs, nil, nil
	}
	match := seatPricePattern.FindStringSubmatch(priceAndCurrency)
	if len(match) != 3 {
		return attrs, nil, nil
	}
	price, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return attrs, nil, nil
	}
	currency := match[2]
	return attrs, &price, &currency
}

func deriveSeatType(attrs map[string]struct{}) string {
	if hasAttr(attrs, "W") {
		return SeatTypeWindow
	}
	if hasAttr(attrs, "A") {
		return SeatTypeAisle
	}
	if hasAttr(attrs, "C") {
		return SeatTypeCentre
	}
	if hasAttr(attrs, "N") {
		return SeatTypeMiddle
	}
	if hasAttr(attrs, "EL") {
		return SeatTypeExtraLegroom
	}
	if hasAttr(attrs, "E") {
		return SeatTypeExitRow
	}
	return SeatTypeStandard
}

func buildSeatDescription(attrs map[string]struct{}) *string {
	descriptions := make([]string, 0)
	for _, attr := range []string{"E", "EL", "K", "B", "H", "PS", "G", "Q", "UF", "WG"} {
		if !hasAttr(attrs, attr) {
			continue
		}
		switch attr {
		case "E":
			descriptions = append(descriptions, "Exit row")
		case "EL":
			descriptions = append(descriptions, "Extra legroom")
		case "K":
			descriptions = append(descriptions, "Bulkhead")
		case "B":
			descriptions = append(descriptions, "Bassinet")
		case "H":
			descriptions = append(descriptions, "Handicapped accessible")
		case "PS":
			descriptions = append(descriptions, "Preferred")
		case "G":
			descriptions = append(descriptions, "Comfort")
		case "Q":
			descriptions = append(descriptions, "Quiet zone")
		case "UF":
			descriptions = append(descriptions, "Up front")
		case "WG":
			descriptions = append(descriptions, "Over wing")
		}
	}
	if len(descriptions) == 0 {
		return nil
	}
	description := strings.Join(descriptions, ", ")
	return &description
}

func hasAttr(attrs map[string]struct{}, attr string) bool {
	_, ok := attrs[attr]
	return ok
}

func hasAnyAttr(attrs map[string]struct{}, candidates map[string]struct{}) bool {
	for attr := range candidates {
		if hasAttr(attrs, attr) {
			return true
		}
	}
	return false
}

func (s *Service) normalizeSeatPrices(ctx context.Context, seatMaps []session.SegmentSeatMap) {
	if s.currency == nil || s.defaultCurrency == "" {
		return
	}
	for i := range seatMaps {
		for j := range seatMaps[i].Seats {
			seat := &seatMaps[i].Seats[j]
			if seat.Price == nil || seat.CurrencyCode == nil {
				continue
			}
			converted, err := s.currency.Convert(ctx, *seat.Price, *seat.CurrencyCode, s.defaultCurrency)
			if err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to convert seat price", "from", *seat.CurrencyCode, "to", s.defaultCurrency, "error", err)
				}
				continue
			}
			seat.Price = &converted
			currencyCode := s.defaultCurrency
			seat.CurrencyCode = &currencyCode
		}
	}
}
