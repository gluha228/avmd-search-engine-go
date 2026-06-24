package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const tfOfferIDPrefix = "TF-"

type parsedTFOfferID struct {
	outwardID string
	returnID  string
}

var (
	luggageQuantityPattern = regexp.MustCompile(`(?i)(\d+)\s+bags?\s*-`)
	luggageWeightKGPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*kg`)
	luggagePricePattern    = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*([A-Z]{3})\s*$`)
)

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

	searchSession, err := s.sessionStore.Get(ctx, searchID)
	if err != nil {
		return nil, fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, searchID)
	}
	if len(searchSession.TFOffers) == 0 {
		return nil, fmt.Errorf("%w: no TravelFusion offers in session: %s", ErrNotFound, searchID)
	}

	offer, ok := findOffer(searchSession.TFOffers, offerID)
	if !ok {
		return nil, fmt.Errorf("%w: offer with ID %s not found in TravelFusion session", ErrNotFound, offerID)
	}
	ids, ok := parseTFOfferID(offerID)
	if !ok {
		return nil, fmt.Errorf("%w: cannot parse TravelFusion outward/return ids from offerId=%s", ErrInvalidRequest, offerID)
	}
	if strings.TrimSpace(searchSession.TFRoutingID) == "" {
		return nil, fmt.Errorf("TravelFusion routing id is missing for ancillary enrichment (offerId=%s)", offerID)
	}

	details, err := s.tfClient.ProcessDetails(ctx, travelfusion.ProcessDetailsRequest{
		RoutingID: searchSession.TFRoutingID,
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
	searchSession.SelectedOfferID = offerID
	searchSession.TFRequiredParameters = required
	s.cacheSeatMap(ctx, searchSession, offerID, offer, required)
	if err := s.sessionStore.Save(ctx, searchID, *searchSession); err != nil {
		return nil, fmt.Errorf("update selected offer session: %w", err)
	}

	offer, err = s.convertOfferToDefaultCurrency(ctx, offer)
	if err != nil {
		return nil, err
	}
	return &SelectedOffer{
		Offer:            offer,
		SearchParams:     searchSession.Params,
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

	searchSession, err := s.sessionStore.Get(ctx, searchID)
	if err != nil {
		return nil, fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, searchID)
	}
	if searchSession.TFSeatMapByOfferID == nil {
		return nil, fmt.Errorf("%w: TravelFusion seat map for offer %s not found in session %s", ErrNotFound, offerID, searchID)
	}
	seatMap, ok := searchSession.TFSeatMapByOfferID[offerID]
	if !ok {
		return nil, fmt.Errorf("%w: TravelFusion seat map for offer %s not found in session %s", ErrNotFound, offerID, searchID)
	}
	return seatMap, nil
}

func findOffer(offers []Offer, offerID string) (Offer, bool) {
	for _, offer := range offers {
		if offer.OfferID == offerID {
			return offer, true
		}
	}
	return Offer{}, false
}

func parseTFOfferID(offerID string) (parsedTFOfferID, bool) {
	if !strings.HasPrefix(offerID, tfOfferIDPrefix) {
		return parsedTFOfferID{}, false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(offerID, tfOfferIDPrefix))
	if raw == "" {
		return parsedTFOfferID{}, false
	}
	outwardID, returnID, hasReturn := strings.Cut(raw, "-")
	if strings.TrimSpace(outwardID) == "" {
		return parsedTFOfferID{}, false
	}
	if hasReturn && strings.TrimSpace(returnID) == "" {
		return parsedTFOfferID{outwardID: outwardID}, true
	}
	return parsedTFOfferID{outwardID: outwardID, returnID: returnID}, true
}

func normalizeRequiredParameters(raw []travelfusion.RequiredParameter) []session.TFRequiredParameterSnapshot {
	result := make([]session.TFRequiredParameterSnapshot, 0, len(raw))
	for _, parameter := range raw {
		code, ok := knownSupplierParameterCode(parameter.Name)
		if !ok {
			continue
		}
		result = append(result, session.TFRequiredParameterSnapshot{
			Parameter:           code,
			Value:               parameter.Value,
			Type:                parameter.Type,
			PerPassenger:        parameter.PerPassenger,
			IsOptional:          parameter.IsOptional,
			IsSometimesRequired: parameter.IsSometimesRequired,
			DisplayText:         parameter.DisplayText,
		})
	}
	return result
}

func (s *Service) mapAdditionalFields(ctx context.Context, required []session.TFRequiredParameterSnapshot) []session.AdditionalField {
	fields := make([]session.AdditionalField, 0, len(required))
	for _, parameter := range required {
		field, ok := s.mapAdditionalField(ctx, parameter)
		if ok {
			fields = append(fields, field)
		}
	}
	return fields
}

func (s *Service) mapAdditionalField(ctx context.Context, parameter session.TFRequiredParameterSnapshot) (session.AdditionalField, bool) {
	switch parameter.Parameter {
	case "PASSPORT_NUMBER":
		if isOptional(parameter) {
			return session.AdditionalField{}, false
		}
		return textAdditionalField(parameter, "TEXT", true), true
	case "PASSPORT_EXPIRY_DATE":
		if isOptional(parameter) {
			return session.AdditionalField{}, false
		}
		return textAdditionalField(parameter, "FORMATTED_TEXT", true), true
	case "LUGGAGE_OPTIONS", "OUTWARD_LUGGAGE_OPTIONS", "RETURN_LUGGAGE_OPTIONS", "HAND_LUGGAGE_OPTIONS":
		if strings.TrimSpace(parameter.DisplayText) == "" {
			return session.AdditionalField{}, false
		}
		inputType := parameter.Type
		if inputType == "" {
			inputType = "VALUE_SELECT"
		}
		return session.AdditionalField{
			Code:         parameter.Parameter,
			InputType:    inputType,
			Required:     !isOptional(parameter),
			PerPassenger: boolValue(parameter.PerPassenger),
			Options:      s.parseLuggageOptions(ctx, parameter.DisplayText),
		}, true
	case "SEAT_OPTIONS":
		if !hasSeatOptions(parameter.DisplayText) {
			return session.AdditionalField{}, false
		}
		return session.AdditionalField{
			Code:         parameter.Parameter,
			Required:     !isOptional(parameter),
			PerPassenger: boolValue(parameter.PerPassenger),
			Options:      []session.AdditionalFieldOption{},
		}, true
	default:
		return session.AdditionalField{}, false
	}
}

func textAdditionalField(parameter session.TFRequiredParameterSnapshot, inputType string, required bool) session.AdditionalField {
	return session.AdditionalField{
		Code:         parameter.Parameter,
		Description:  strings.TrimSpace(parameter.DisplayText),
		InputType:    inputType,
		Required:     required,
		PerPassenger: boolValue(parameter.PerPassenger),
		Options:      []session.AdditionalFieldOption{},
	}
}

func (s *Service) parseLuggageOptions(ctx context.Context, displayText string) []session.AdditionalFieldOption {
	_, rawOptions, ok := strings.Cut(displayText, ":")
	if !ok {
		return nil
	}
	tokens := strings.Split(rawOptions, ",")
	options := make([]session.AdditionalFieldOption, 0, len(tokens))
	seen := map[string]struct{}{}
	for _, token := range tokens {
		value, label := parseLuggageOptionToken(token)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		parsed, ok := parseLuggageInner(label)
		if ok {
			label = formatLuggageDescriptor(localeFromContext(ctx), parsed)
		}
		option := session.AdditionalFieldOption{Value: value, Label: label}
		if ok {
			option.Price = s.luggageOptionPrice(ctx, parsed.Price, parsed.CurrencyCode)
		}
		options = append(options, option)
	}
	return options
}

func parseLuggageOptionToken(token string) (string, string) {
	option := strings.TrimSpace(token)
	if option == "" {
		return "", ""
	}
	openBracket := strings.Index(option, "(")
	closeBracket := strings.LastIndex(option, ")")
	if openBracket <= 0 || closeBracket <= openBracket {
		return option, option
	}
	value := strings.TrimSpace(option[:openBracket])
	label := strings.TrimSpace(option[openBracket+1 : closeBracket])
	if value == "" {
		return "", ""
	}
	return value, label
}

type parsedLuggageOption struct {
	Quantity      int
	WeightPartsKG []string
	Price         float64
	CurrencyCode  string
}

func parseLuggageInner(inner string) (parsedLuggageOption, bool) {
	if strings.TrimSpace(inner) == "" {
		return parsedLuggageOption{}, false
	}
	qtyMatch := luggageQuantityPattern.FindStringSubmatch(inner)
	priceMatch := luggagePricePattern.FindStringSubmatch(inner)
	if len(qtyMatch) != 2 || len(priceMatch) != 3 {
		return parsedLuggageOption{}, false
	}
	quantity, err := strconv.Atoi(qtyMatch[1])
	if err != nil {
		return parsedLuggageOption{}, false
	}
	price, err := strconv.ParseFloat(priceMatch[1], 64)
	if err != nil {
		return parsedLuggageOption{}, false
	}
	weightMatches := luggageWeightKGPattern.FindAllStringSubmatch(inner, -1)
	if len(weightMatches) == 0 {
		return parsedLuggageOption{}, false
	}
	weights := make([]string, 0, len(weightMatches))
	for _, match := range weightMatches {
		if len(match) == 2 {
			weights = append(weights, trimTrailingZeros(match[1]))
		}
	}
	if len(weights) == 0 {
		return parsedLuggageOption{}, false
	}
	return parsedLuggageOption{
		Quantity:      quantity,
		WeightPartsKG: weights,
		Price:         price,
		CurrencyCode:  strings.ToUpper(priceMatch[2]),
	}, true
}

func formatLuggageDescriptor(locale string, parsed parsedLuggageOption) string {
	return formatLuggageQuantity(locale, parsed.Quantity) + " - " + formatLuggageDistribution(locale, parsed.WeightPartsKG)
}

func formatLuggageQuantity(locale string, quantity int) string {
	switch normalizeLocale(locale) {
	case "ru":
		if quantity == 1 {
			return "1 багаж"
		}
		return strconv.Itoa(quantity) + " багажа"
	case "ro":
		if quantity == 1 {
			return "1 bagaj"
		}
		return strconv.Itoa(quantity) + " bagaje"
	default:
		return strconv.Itoa(quantity) + " bags"
	}
}

func formatLuggageDistribution(locale string, weightParts []string) string {
	unit := "kg"
	if normalizeLocale(locale) == "ru" {
		unit = "кг"
	}
	parts := make([]string, 0, len(weightParts))
	for _, weight := range weightParts {
		if strings.TrimSpace(weight) != "" {
			parts = append(parts, strings.TrimSpace(weight)+" "+unit)
		}
	}
	return strings.Join(parts, " + ")
}

func trimTrailingZeros(value string) string {
	if !strings.Contains(value, ".") {
		return value
	}
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	return value
}

func (s *Service) luggageOptionPrice(ctx context.Context, amount float64, currencyCode string) *session.AdditionalFieldOptionPrice {
	if s.currency != nil && s.defaultCurrency != "" {
		converted, err := s.currency.Convert(ctx, amount, currencyCode, s.defaultCurrency)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to convert luggage option price", "from", currencyCode, "to", s.defaultCurrency, "error", err)
			}
			return nil
		}
		amount = converted
		currencyCode = s.defaultCurrency
	}
	return &session.AdditionalFieldOptionPrice{Amount: amount, CurrencyCode: currencyCode}
}

func isOptional(parameter session.TFRequiredParameterSnapshot) bool {
	return boolValue(parameter.IsOptional)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func localeFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(localeContextKey{}).(string); ok {
		return normalizeLocale(value)
	}
	return "en"
}

func normalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if strings.HasPrefix(locale, "ru") {
		return "ru"
	}
	if strings.HasPrefix(locale, "ro") {
		return "ro"
	}
	return "en"
}

func knownSupplierParameterCode(name string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	code, ok := knownSupplierParameterByTFName[normalized]
	return code, ok
}

var knownSupplierParameterByTFName = map[string]string{
	"dateofbirth":               "DATE_OF_BIRTH",
	"passportnumber":            "PASSPORT_NUMBER",
	"passportexpirydate":        "PASSPORT_EXPIRY_DATE",
	"passportcountryofissue":    "PASSPORT_COUNTRY_OF_ISSUE",
	"nationality":               "NATIONALITY",
	"fullname":                  "FULL_NAME",
	"firstnameandlastname":      "FIRST_NAME_AND_LAST_NAME",
	"firstname":                 "FIRST_NAME",
	"titleandfirstname":         "TITLE_AND_FIRST_NAME",
	"lastname":                  "LAST_NAME",
	"title":                     "TITLE",
	"email":                     "EMAIL",
	"bookingmobilephone":        "BOOKING_MOBILE_PHONE",
	"luggageoptions":            "LUGGAGE_OPTIONS",
	"outwardluggageoptions":     "OUTWARD_LUGGAGE_OPTIONS",
	"returnluggageoptions":      "RETURN_LUGGAGE_OPTIONS",
	"handluggageoptions":        "HAND_LUGGAGE_OPTIONS",
	"seatoptions":               "SEAT_OPTIONS",
	"usetfprepay":               "USE_TF_PREPAY",
	"cardsecuritynumber":        "CARD_SECURITY_NUMBER",
	"childrenandinfantssearch":  "CHILDREN_AND_INFANTS_SEARCH",
	"childrenandinfantsbooking": "CHILDREN_AND_INFANTS_BOOKING",
	"bookingsourceref":          "BOOKING_SOURCE_REF",
	"fullcardnamebreakdown":     "FULL_CARD_NAME_BREAKDOWN",
	"postcode":                  "POST_CODE",
	"billingaddress":            "BILLING_ADDRESS",
	"titlessupported":           "TITLES_SUPPORTED",
	"bookingonhold":             "BOOKING_ON_HOLD",
}
