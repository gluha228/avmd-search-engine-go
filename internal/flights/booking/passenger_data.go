package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var passportNumberPattern = regexp.MustCompile(`^[A-Za-z0-9<]{5,32}$`)

func (s *Service) ProcessPassengerData(ctx context.Context, req PassengerDataRequest) (*PassengerDataResponse, error) {
	req.SearchID = strings.TrimSpace(req.SearchID)
	req.OfferID = strings.TrimSpace(req.OfferID)
	if req.SearchID == "" {
		return nil, fmt.Errorf("%w: search_id is required", ErrInvalidRequest)
	}
	if req.OfferID == "" {
		return nil, fmt.Errorf("%w: offer_id is required", ErrInvalidRequest)
	}
	if !strings.HasPrefix(req.OfferID, tfOfferIDPrefix) {
		return nil, fmt.Errorf("%w: ProcessTerms is only supported for TravelFusion offers (offer_id=%s)", ErrInvalidRequest, req.OfferID)
	}
	if s.sessionStore == nil {
		return nil, fmt.Errorf("%w: session store is not configured", ErrNotFound)
	}

	searchSession, err := s.sessionStore.Get(ctx, req.SearchID)
	if err != nil {
		return nil, fmt.Errorf("%w: search session expired or not found for ID: %s", ErrNotFound, req.SearchID)
	}
	expectedPassengers := searchSession.Params.AdultCount + searchSession.Params.ChildCount + searchSession.Params.InfantCount
	if expectedPassengers <= 0 {
		return nil, fmt.Errorf("%w: search session has no passenger count", ErrInvalidRequest)
	}
	if len(req.Passengers) != expectedPassengers {
		return nil, fmt.Errorf("%w: passenger list size must equal total passengers from the search (%d)", ErrInvalidRequest, expectedPassengers)
	}
	if _, ok := findOffer(searchSession.TFOffers, req.OfferID); !ok {
		return nil, fmt.Errorf("%w: offer with ID %s not found in TravelFusion session", ErrNotFound, req.OfferID)
	}
	if strings.TrimSpace(searchSession.TFRoutingID) == "" {
		return nil, fmt.Errorf("TravelFusion routing id is missing")
	}
	ids, ok := parseTFOfferID(req.OfferID)
	if !ok {
		return nil, fmt.Errorf("%w: cannot parse TravelFusion outward/return ids from offer_id=%s", ErrInvalidRequest, req.OfferID)
	}
	if err := validatePassengerData(req, searchSession); err != nil {
		return nil, err
	}

	profile, err := s.buildProcessTermsProfile(req, searchSession)
	if err != nil {
		return nil, err
	}
	tfResp, err := s.tfClient.ProcessTerms(ctx, travelfusion.ProcessTermsRequest{
		RoutingID:      searchSession.TFRoutingID,
		OutwardID:      ids.outwardID,
		ReturnID:       ids.returnID,
		BookingProfile: profile,
	})
	if err != nil {
		return nil, fmt.Errorf("TravelFusion ProcessTerms failed (offer_id=%s): %w", req.OfferID, err)
	}
	return mapPassengerDataResponse(tfResp), nil
}

func (s *Service) buildProcessTermsProfile(req PassengerDataRequest, searchSession *session.FlightSearchSession) (travelfusion.BookingProfile, error) {
	travellers := make([]travelfusion.Traveller, len(req.Passengers))
	for i := range req.Passengers {
		travellers[i] = travelfusion.Traveller{
			Age:  passengerAge(req.Passengers[i].DateOfBirth, s.now()),
			Name: passengerName(req.Passengers[i]),
		}
	}
	profile := travelfusion.BookingProfile{
		Travellers: travellers,
		ContactDetails: travelfusion.ContactDetails{
			Name:        passengerName(req.Passengers[0]),
			Address:     minimalAddress(req.Passengers[0].CitizenshipCountryCode),
			MobilePhone: processTermsPhone(req.ContactData.Phone),
			Email:       strings.TrimSpace(req.ContactData.Email),
		},
		CustomSupplierParameters: []travelfusion.CustomSupplierParameter{
			{Name: "UseTFPrepay", Value: "Always"},
		},
	}

	if req.OfferID == searchSession.SelectedOfferID {
		if err := applyProcessTermsSnapshots(&profile, req, searchSession.TFRequiredParameters); err != nil {
			return travelfusion.BookingProfile{}, err
		}
	}
	return profile, nil
}

func applyProcessTermsSnapshots(profile *travelfusion.BookingProfile, req PassengerDataRequest, snapshots []session.TFRequiredParameterSnapshot) error {
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.Parameter) == "" {
			continue
		}
		switch snapshot.Parameter {
		case "DATE_OF_BIRTH":
			if isOptional(snapshot) {
				continue
			}
			for i := range req.Passengers {
				addTravellerCSP(profile, i, "DateOfBirth", req.Passengers[i].DateOfBirth.Format("02/01/2006"))
			}
		case "EMAIL":
			if isOptional(snapshot) {
				continue
			}
			email := strings.TrimSpace(req.ContactData.Email)
			if email == "" {
				return fmt.Errorf("%w: contact email is required for supplier parameter Email", ErrInvalidRequest)
			}
			addBookingCSP(profile, "Email", email)
		case "BOOKING_MOBILE_PHONE":
			if isOptional(snapshot) {
				continue
			}
			phone := bookingMobilePhoneSupplierParameter(req.ContactData.Phone)
			if phone == "" {
				return fmt.Errorf("%w: contact phone is required for supplier parameter BookingMobilePhone", ErrInvalidRequest)
			}
			addBookingCSP(profile, "BookingMobilePhone", phone)
		case "FULL_NAME", "FIRST_NAME_AND_LAST_NAME", "FIRST_NAME", "TITLE_AND_FIRST_NAME", "LAST_NAME", "TITLE":
			if isOptional(snapshot) {
				continue
			}
			if err := applyNameCSP(profile, req, snapshot); err != nil {
				return err
			}
		default:
			if isListBackedProcessTermsCSP(snapshot.Parameter) {
				if err := applyListBackedCSP(profile, req, snapshot); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isListBackedProcessTermsCSP(parameter string) bool {
	switch parameter {
	case "PASSPORT_NUMBER", "PASSPORT_EXPIRY_DATE", "PASSPORT_COUNTRY_OF_ISSUE", "NATIONALITY",
		"LUGGAGE_OPTIONS", "OUTWARD_LUGGAGE_OPTIONS", "RETURN_LUGGAGE_OPTIONS", "HAND_LUGGAGE_OPTIONS", "SEAT_OPTIONS":
		return true
	default:
		return false
	}
}

func applyListBackedCSP(profile *travelfusion.BookingProfile, req PassengerDataRequest, snapshot session.TFRequiredParameterSnapshot) error {
	wireName, ok := knownSupplierParameterTFName(snapshot.Parameter)
	if !ok {
		return nil
	}
	optional := isOptional(snapshot)
	if boolValue(snapshot.PerPassenger) {
		for i := range req.Passengers {
			value, ok := firstPassengerSupplierParameter(req, i, snapshot.Parameter)
			if !ok && optional {
				continue
			}
			if !ok {
				return fmt.Errorf("%w: missing required CustomSupplierParameter %s for passenger index %d", ErrInvalidRequest, wireName, i)
			}
			addTravellerCSP(profile, i, wireName, value)
		}
		return nil
	}
	value, ok := firstBookingSupplierParameter(req, snapshot.Parameter)
	if !ok && optional {
		return nil
	}
	if !ok {
		return fmt.Errorf("%w: missing required booking-level CustomSupplierParameter %s", ErrInvalidRequest, wireName)
	}
	addBookingCSP(profile, wireName, value)
	return nil
}

func applyNameCSP(profile *travelfusion.BookingProfile, req PassengerDataRequest, snapshot session.TFRequiredParameterSnapshot) error {
	wireName, ok := knownSupplierParameterTFName(snapshot.Parameter)
	if !ok {
		return nil
	}
	if boolValue(snapshot.PerPassenger) {
		for i := range req.Passengers {
			value := buildNameCSPValue(snapshot.Parameter, req.Passengers[i])
			if value == "" {
				return fmt.Errorf("%w: passenger index %d: cannot build required TF CustomSupplierParameter %s from passenger name", ErrInvalidRequest, i, wireName)
			}
			addTravellerCSP(profile, i, wireName, value)
		}
		return nil
	}
	value := buildNameCSPValue(snapshot.Parameter, req.Passengers[0])
	if value == "" {
		return fmt.Errorf("%w: cannot build required booking-level TF CustomSupplierParameter %s from lead passenger name", ErrInvalidRequest, wireName)
	}
	addBookingCSP(profile, wireName, value)
	return nil
}

func buildNameCSPValue(parameter string, passenger session.Passenger) string {
	title := normalizeTitle(passenger.Title)
	first := normalizeNamePart(passenger.FirstName)
	last := normalizeNamePart(passenger.LastName)
	switch parameter {
	case "FULL_NAME":
		if title == "" || first == "" || last == "" {
			return ""
		}
		return title + "," + first + "," + last
	case "FIRST_NAME_AND_LAST_NAME":
		if first == "" || last == "" {
			return ""
		}
		return first + "," + last
	case "FIRST_NAME":
		return first
	case "TITLE_AND_FIRST_NAME":
		if title == "" || first == "" {
			return ""
		}
		return title + "," + first
	case "LAST_NAME":
		return last
	case "TITLE":
		return title
	default:
		return ""
	}
}

func validatePassengerData(req PassengerDataRequest, searchSession *session.FlightSearchSession) error {
	if strings.TrimSpace(req.ContactData.Email) == "" {
		return fmt.Errorf("%w: contact_data.email is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.ContactData.Phone.InternationalCode) == "" || strings.TrimSpace(req.ContactData.Phone.Number) == "" {
		return fmt.Errorf("%w: contact_data.phone is required", ErrInvalidRequest)
	}
	for i := range req.Passengers {
		passenger := req.Passengers[i]
		if normalizeTitle(passenger.Title) == "" {
			return fmt.Errorf("%w: passengers[%d].title must be Mr, Mrs or Miss", ErrInvalidRequest, i)
		}
		if normalizeNamePart(passenger.FirstName) == "" || normalizeNamePart(passenger.LastName) == "" {
			return fmt.Errorf("%w: passengers[%d].first_name and last_name are required", ErrInvalidRequest, i)
		}
		if passenger.DateOfBirth.IsZero() {
			return fmt.Errorf("%w: passengers[%d].date_of_birth is required", ErrInvalidRequest, i)
		}
		if !isISO2(passenger.CitizenshipCountryCode) {
			return fmt.Errorf("%w: passengers[%d].citizenship_country_code must be ISO 3166-1 alpha-2 uppercase", ErrInvalidRequest, i)
		}
	}
	if req.OfferID == searchSession.SelectedOfferID {
		return validateRequiredDocumentCSPs(req, searchSession.TFRequiredParameters)
	}
	return nil
}

func validateRequiredDocumentCSPs(req PassengerDataRequest, snapshots []session.TFRequiredParameterSnapshot) error {
	for _, snapshot := range snapshots {
		if isOptional(snapshot) {
			continue
		}
		switch snapshot.Parameter {
		case "PASSPORT_NUMBER":
			if err := validateRequiredSupplierParam(req, snapshot, validatePassportNumberValue); err != nil {
				return err
			}
		case "PASSPORT_EXPIRY_DATE":
			if err := validateRequiredSupplierParam(req, snapshot, validatePassportExpiryValue); err != nil {
				return err
			}
		case "PASSPORT_COUNTRY_OF_ISSUE", "NATIONALITY":
			if err := validateRequiredSupplierParam(req, snapshot, func(value string) error {
				if !isISO2(value) {
					return fmt.Errorf("must be ISO 3166-1 alpha-2 uppercase")
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRequiredSupplierParam(req PassengerDataRequest, snapshot session.TFRequiredParameterSnapshot, validate func(string) error) error {
	if boolValue(snapshot.PerPassenger) {
		for i := range req.Passengers {
			value, ok := firstPassengerSupplierParameter(req, i, snapshot.Parameter)
			if !ok {
				return fmt.Errorf("%w: missing required %s for passenger index %d", ErrInvalidRequest, snapshot.Parameter, i)
			}
			if err := validate(value); err != nil {
				return fmt.Errorf("%w: passenger index %d %s %s", ErrInvalidRequest, i, snapshot.Parameter, err.Error())
			}
		}
		return nil
	}
	value, ok := firstBookingSupplierParameter(req, snapshot.Parameter)
	if !ok {
		return fmt.Errorf("%w: missing required booking-level %s", ErrInvalidRequest, snapshot.Parameter)
	}
	if err := validate(value); err != nil {
		return fmt.Errorf("%w: booking-level %s %s", ErrInvalidRequest, snapshot.Parameter, err.Error())
	}
	return nil
}

func validatePassportNumberValue(value string) error {
	if !passportNumberPattern.MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("must be 5-32 chars and contain only letters, digits or '<'")
	}
	return nil
}

func validatePassportExpiryValue(value string) error {
	parsed, err := time.Parse("02/01/2006", strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("must be a valid TF date dd/MM/yyyy")
	}
	if !parsed.After(time.Now()) {
		return fmt.Errorf("must be in the future")
	}
	return nil
}

func passengerAge(dateOfBirth, now time.Time) int {
	years := now.Year() - dateOfBirth.Year()
	if now.Month() < dateOfBirth.Month() || (now.Month() == dateOfBirth.Month() && now.Day() < dateOfBirth.Day()) {
		years--
	}
	return years
}

func passengerName(passenger session.Passenger) travelfusion.Name {
	return travelfusion.Name{
		Title:     normalizeTitle(passenger.Title),
		NameParts: []string{strings.TrimSpace(passenger.FirstName), strings.TrimSpace(passenger.LastName)},
	}
}

func minimalAddress(countryCode string) travelfusion.Address {
	return travelfusion.Address{
		City:        "NA",
		Street:      "NA",
		CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)),
		Postcode:    "NONE",
		Province:    "OT",
	}
}

func processTermsPhone(phone session.Phone) travelfusion.Phone {
	return travelfusion.Phone{
		InternationalCode: internationalCodeDigitsForTF(phone.InternationalCode),
		Number:            subscriberDigits(phone.Number),
	}
}

func internationalCodeDigitsForTF(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "+") {
		trimmed = "00" + strings.TrimPrefix(trimmed, "+")
	}
	return digitsOnly(trimmed)
}

func subscriberDigits(raw string) string {
	return digitsOnly(strings.TrimSpace(raw))
}

func bookingMobilePhoneSupplierParameter(phone session.Phone) string {
	return internationalCodeDigitsForTF(phone.InternationalCode) + subscriberDigits(phone.Number)
}

func digitsOnly(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func firstPassengerSupplierParameter(req PassengerDataRequest, index int, paramName string) (string, bool) {
	if index < 0 || index >= len(req.Passengers) {
		return "", false
	}
	return firstSupplierParameter(req.Passengers[index].SupplierParameters, paramName)
}

func firstBookingSupplierParameter(req PassengerDataRequest, paramName string) (string, bool) {
	return firstSupplierParameter(req.SupplierParameters, paramName)
}

func firstSupplierParameter(params []session.SupplierParameter, paramName string) (string, bool) {
	for _, param := range params {
		if strings.EqualFold(strings.TrimSpace(param.ParamName), paramName) {
			value := strings.TrimSpace(param.ParamValue)
			if value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func addTravellerCSP(profile *travelfusion.BookingProfile, passengerIndex int, name, value string) {
	if passengerIndex < 0 || passengerIndex >= len(profile.Travellers) || strings.TrimSpace(value) == "" {
		return
	}
	profile.Travellers[passengerIndex].CustomSupplierParameters = append(profile.Travellers[passengerIndex].CustomSupplierParameters, travelfusion.CustomSupplierParameter{Name: name, Value: value})
}

func addBookingCSP(profile *travelfusion.BookingProfile, name, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	profile.CustomSupplierParameters = append(profile.CustomSupplierParameters, travelfusion.CustomSupplierParameter{Name: name, Value: value})
}

func normalizeTitle(title string) string {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "mr":
		return "Mr"
	case "mrs":
		return "Mrs"
	case "miss":
		return "Miss"
	default:
		return ""
	}
}

func normalizeNamePart(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func isISO2(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == 2 && value == strings.ToUpper(value) && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func mapPassengerDataResponse(src *travelfusion.ProcessTermsResult) *PassengerDataResponse {
	if src == nil {
		return &PassengerDataResponse{SupplierResponses: []session.ProcessTermsSupplierResponse{}}
	}
	responses := make([]session.ProcessTermsSupplierResponse, len(src.SupplierResponses))
	for i := range src.SupplierResponses {
		responses[i] = session.ProcessTermsSupplierResponse{
			Name: src.SupplierResponses[i].Name,
			Type: src.SupplierResponses[i].Type,
			Data: src.SupplierResponses[i].Data,
		}
	}
	return &PassengerDataResponse{
		RoutingID:                           src.RoutingID,
		TFBookingReference:                  src.TFBookingReference,
		FinalAmount:                         src.FinalAmount,
		FinalCurrency:                       src.FinalCurrency,
		SupplierVisualAuthorisationImageURL: src.SupplierVisualAuthorisationImageURL,
		SupplierResponses:                   responses,
	}
}

func knownSupplierParameterTFName(parameter string) (string, bool) {
	name, ok := knownSupplierParameterTFNames[strings.ToUpper(strings.TrimSpace(parameter))]
	return name, ok
}

var knownSupplierParameterTFNames = map[string]string{
	"DATE_OF_BIRTH":             "DateOfBirth",
	"PASSPORT_NUMBER":           "PassportNumber",
	"PASSPORT_EXPIRY_DATE":      "PassportExpiryDate",
	"PASSPORT_COUNTRY_OF_ISSUE": "PassportCountryOfIssue",
	"NATIONALITY":               "Nationality",
	"FULL_NAME":                 "FullName",
	"FIRST_NAME_AND_LAST_NAME":  "FirstNameAndLastName",
	"FIRST_NAME":                "FirstName",
	"TITLE_AND_FIRST_NAME":      "TitleAndFirstName",
	"LAST_NAME":                 "LastName",
	"TITLE":                     "Title",
	"EMAIL":                     "Email",
	"BOOKING_MOBILE_PHONE":      "BookingMobilePhone",
	"LUGGAGE_OPTIONS":           "LuggageOptions",
	"OUTWARD_LUGGAGE_OPTIONS":   "OutwardLuggageOptions",
	"RETURN_LUGGAGE_OPTIONS":    "ReturnLuggageOptions",
	"HAND_LUGGAGE_OPTIONS":      "HandLuggageOptions",
	"SEAT_OPTIONS":              "SeatOptions",
	"USE_TF_PREPAY":             "UseTFPrepay",
}
