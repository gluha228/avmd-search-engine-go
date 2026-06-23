package flights

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTFClient struct {
	result         *travelfusion.SearchResult
	processDetails *travelfusion.ProcessDetailsResult
	err            error
}

type fakeSessionStore struct {
	searchID string
	session  FlightSearchSession
	err      error
}

type fakeCalendarCache struct {
	calls []calendarCacheCall
}

type fakeCurrencyConverter struct {
	from    string
	to      string
	amount  float64
	result  float64
	results []float64
	calls   []currencyConversionCall
	err     error
}

type currencyConversionCall struct {
	amount float64
	from   string
	to     string
}

type calendarCacheCall struct {
	departure string
	arrival   string
	flights   []travelfusion.Flight
}

func (f fakeTFClient) Search(context.Context, travelfusion.SearchRequest) (*travelfusion.SearchResult, error) {
	return f.result, f.err
}

func (f fakeTFClient) ProcessDetails(context.Context, travelfusion.ProcessDetailsRequest) (*travelfusion.ProcessDetailsResult, error) {
	return f.processDetails, f.err
}

func (f *fakeSessionStore) Create(_ context.Context, session FlightSearchSession) (string, error) {
	f.session = session
	return f.searchID, f.err
}

func (f *fakeSessionStore) Save(_ context.Context, _ string, session FlightSearchSession) error {
	f.session = session
	return f.err
}

func (f *fakeSessionStore) Get(_ context.Context, _ string) (*FlightSearchSession, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.session, nil
}

func (f *fakeCalendarCache) CacheFlights(_ context.Context, departure, arrival string, flights []travelfusion.Flight) error {
	f.calls = append(f.calls, calendarCacheCall{departure: departure, arrival: arrival, flights: flights})
	return nil
}

func (f *fakeCurrencyConverter) Convert(_ context.Context, amount float64, from, to string) (float64, error) {
	f.amount = amount
	f.from = from
	f.to = to
	f.calls = append(f.calls, currencyConversionCall{amount: amount, from: from, to: to})
	if f.err != nil {
		return 0, f.err
	}
	if len(f.results) > 0 {
		result := f.results[0]
		f.results = f.results[1:]
		return result, nil
	}
	if f.result != 0 {
		return f.result, nil
	}
	return amount, nil
}

func TestSearchOneWayFiltersToDepartureDate(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	service := NewService(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID: "RID",
		OutwardFlights: []travelfusion.Flight{
			tfFlight("OUT1", "KIV", "OTP", departure, 100),
			tfFlight("OUT2", "KIV", "OTP", departure.AddDate(0, 0, 1), 50),
		},
	}}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        departure,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.RoutingID != "RID" {
		t.Fatalf("unexpected routing id: %s", resp.RoutingID)
	}
	if len(resp.Offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(resp.Offers))
	}
	if resp.Offers[0].OfferID != "TF-OUT1" || resp.Offers[0].Price != 100 {
		t.Fatalf("unexpected offer: %+v", resp.Offers[0])
	}
}

func TestSearchCreatesFlightSearchSession(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{searchID: "search-1"}
	service := NewServiceWithSessionStore(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID:      "RID",
		OutwardFlights: []travelfusion.Flight{tfFlight("OUT1", "KIV", "OTP", departure, 100)},
	}}, store, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        departure,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if resp.SearchID != "search-1" {
		t.Fatalf("expected search id from session store, got %q", resp.SearchID)
	}
	if store.session.TFRoutingID != "RID" || len(store.session.TFOffers) != 1 {
		t.Fatalf("unexpected stored session: %+v", store.session)
	}
}

func TestGetSelectedOfferFetchesProcessDetailsAndCachesRequiredParameters(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{searchID: "search-1", session: FlightSearchSession{
		Params: SearchRequest{
			DepartureAirportCode: "KIV",
			ArrivalAirportCode:   "OTP",
			DepartureDate:        departure,
			AdultCount:           1,
		},
		TFRoutingID: "RID",
		TFOffers: []Offer{
			{OfferID: "TF-OUT1", CurrencyCode: "EUR", Price: 100},
		},
	}}
	required := false
	perPassenger := true
	service := NewServiceWithSessionStore(fakeTFClient{processDetails: &travelfusion.ProcessDetailsResult{
		RoutingID: "RID",
		RequiredParameters: []travelfusion.RequiredParameter{
			{
				Name:         "PassportNumber",
				Type:         "TEXT",
				DisplayText:  "Passport number",
				PerPassenger: &perPassenger,
				IsOptional:   &required,
			},
		},
	}}, store, nil)

	selected, err := service.GetSelectedOffer(context.Background(), "search-1", "TF-OUT1")
	if err != nil {
		t.Fatalf("GetSelectedOffer returned error: %v", err)
	}
	if selected.Offer.OfferID != "TF-OUT1" {
		t.Fatalf("unexpected selected offer: %+v", selected.Offer)
	}
	if len(selected.AdditionalFields) != 1 || selected.AdditionalFields[0].Code != "PASSPORT_NUMBER" {
		t.Fatalf("expected passport additional field, got %+v", selected.AdditionalFields)
	}
	if store.session.SelectedOfferID != "TF-OUT1" || len(store.session.TFRequiredParameters) != 1 {
		t.Fatalf("expected selected offer data cached in session, got %+v", store.session)
	}
}

func TestGetSelectedOfferConvertsOfferToDefaultCurrency(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	inbound := Flight{Price: 50}
	store := &fakeSessionStore{searchID: "search-1", session: FlightSearchSession{
		Params:      SearchRequest{DepartureAirportCode: "KIV", ArrivalAirportCode: "OTP", DepartureDate: departure, AdultCount: 1},
		TFRoutingID: "RID",
		TFOffers: []Offer{
			{
				OfferID:        "TF-OUT1-RET1",
				CurrencyCode:   "USD",
				Price:          150,
				OutboundFlight: Flight{Price: 100},
				InboundFlight:  &inbound,
			},
		},
	}}
	converter := &fakeCurrencyConverter{results: []float64{90, 45}}
	service := NewServiceWithBookingDependencies(fakeTFClient{processDetails: &travelfusion.ProcessDetailsResult{
		RoutingID: "RID",
	}}, store, nil, converter, "EUR", nil)

	selected, err := service.GetSelectedOffer(context.Background(), "search-1", "TF-OUT1-RET1")
	if err != nil {
		t.Fatalf("GetSelectedOffer returned error: %v", err)
	}
	if selected.Offer.CurrencyCode != "EUR" || selected.Offer.Price != 135 {
		t.Fatalf("expected selected offer converted to EUR, got %+v", selected.Offer)
	}
	if selected.Offer.OutboundFlight.Price != 90 || selected.Offer.InboundFlight == nil || selected.Offer.InboundFlight.Price != 45 {
		t.Fatalf("expected converted leg prices, got %+v", selected.Offer)
	}
	if len(converter.calls) != 2 || converter.calls[0].from != "USD" || converter.calls[0].to != "EUR" {
		t.Fatalf("unexpected conversion calls: %+v", converter.calls)
	}
}

func TestGetSelectedOfferCachesSeatMapFromProcessDetails(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{searchID: "search-1", session: FlightSearchSession{
		Params:      SearchRequest{DepartureAirportCode: "KIV", ArrivalAirportCode: "OTP", DepartureDate: departure, AdultCount: 1},
		TFRoutingID: "RID",
		TFOffers: []Offer{
			{
				OfferID:      "TF-OUT1",
				CurrencyCode: "EUR",
				Price:        100,
				OutboundFlight: Flight{Segments: []Segment{
					{
						SegmentID:            1,
						DepartureAirportCode: "KIV",
						ArrivalAirportCode:   "OTP",
						FlightNumber:         "TF100",
					},
				}},
			},
		},
	}}
	isOptional := false
	service := NewServiceWithBookingDependencies(fakeTFClient{processDetails: &travelfusion.ProcessDetailsResult{
		RoutingID: "RID",
		RequiredParameters: []travelfusion.RequiredParameter{
			{
				Name:        "SeatOptions",
				Type:        "custom",
				DisplayText: "Please Select Seat Options: TF100-12A(|W@25.00EUR@A320),TF100-12B(|T|A@@)",
				IsOptional:  &isOptional,
			},
		},
	}}, store, nil, &fakeCurrencyConverter{result: 20}, "EUR", nil)

	_, err := service.GetSelectedOffer(context.Background(), "search-1", "TF-OUT1")
	if err != nil {
		t.Fatalf("GetSelectedOffer returned error: %v", err)
	}
	seatMap := store.session.TFSeatMapByOfferID["TF-OUT1"]
	if len(seatMap) != 1 || len(seatMap[0].Seats) != 2 {
		t.Fatalf("expected cached seat map, got %+v", store.session.TFSeatMapByOfferID)
	}
	if seatMap[0].Seats[0].Code != "12A" || seatMap[0].Seats[0].Type != SeatTypeWindow || seatMap[0].Seats[0].Price == nil || *seatMap[0].Seats[0].Price != 20 {
		t.Fatalf("unexpected parsed seat: %+v", seatMap[0].Seats[0])
	}
	if seatMap[0].Seats[1].IsAvailable {
		t.Fatalf("expected T seat to be unavailable: %+v", seatMap[0].Seats[1])
	}
}

func TestGetSeatMapReturnsCachedSeatMap(t *testing.T) {
	store := &fakeSessionStore{session: FlightSearchSession{
		TFSeatMapByOfferID: map[string][]SegmentSeatMap{
			"TF-OUT1": {
				{
					SegmentID:    1,
					Origin:       "KIV",
					Destination:  "OTP",
					FlightNumber: "TF100",
					Seats: []SeatDetail{
						{Code: "12A", Type: SeatTypeWindow, Row: 12, Col: 0, IsAvailable: true},
					},
				},
			},
		},
	}}
	service := NewServiceWithSessionStore(fakeTFClient{}, store, nil)

	seatMap, err := service.GetSeatMap(context.Background(), "search-1", "TF-OUT1")
	if err != nil {
		t.Fatalf("GetSeatMap returned error: %v", err)
	}
	if len(seatMap) != 1 || seatMap[0].Seats[0].Code != "12A" {
		t.Fatalf("unexpected seat map: %+v", seatMap)
	}
}

func TestParseLuggageInnerExtractsTrailingPriceCurrency(t *testing.T) {
	parsed, ok := parseLuggageInner("1 bags - 20Kg total - 25.00 EUR")
	if !ok {
		t.Fatal("expected luggage option to parse")
	}
	if parsed.Quantity != 1 || parsed.Price != 25 || parsed.CurrencyCode != "EUR" {
		t.Fatalf("unexpected parsed luggage option: %+v", parsed)
	}
	if len(parsed.WeightPartsKG) != 1 || parsed.WeightPartsKG[0] != "20" {
		t.Fatalf("unexpected weight parts: %+v", parsed.WeightPartsKG)
	}
}

func TestParseLuggageInnerDoesNotTreatBagAsCurrency(t *testing.T) {
	if _, ok := parseLuggageInner("1 BAG 20 KG"); ok {
		t.Fatal("expected luggage option without trailing price/currency not to parse")
	}
}

func TestMapAdditionalFieldConvertsLuggagePriceCurrency(t *testing.T) {
	converter := &fakeCurrencyConverter{result: 30}
	service := NewServiceWithBookingDependencies(fakeTFClient{}, nil, nil, converter, "EUR", nil)

	field, ok := service.mapAdditionalField(WithLocale(context.Background(), "en"), TFRequiredParameterSnapshot{
		Parameter:   "LUGGAGE_OPTIONS",
		Type:        "VALUE_SELECT",
		DisplayText: "LuggageOptions: 1 (1 bags - 20Kg total - 25.00 EUR)",
	})
	if !ok {
		t.Fatal("expected luggage additional field")
	}
	if converter.from != "EUR" || converter.to != "EUR" || converter.amount != 25 {
		t.Fatalf("unexpected conversion call: %+v", converter)
	}
	if len(field.Options) != 1 || field.Options[0].Price == nil {
		t.Fatalf("expected luggage price option, got %+v", field.Options)
	}
	if field.Options[0].Label != "1 bags - 20 kg" || field.Options[0].Price.Amount != 30 {
		t.Fatalf("unexpected luggage option: %+v", field.Options[0])
	}
}

func TestFormatLuggageDescriptorLocalizesLikeJava(t *testing.T) {
	parsed := parsedLuggageOption{
		Quantity:      2,
		WeightPartsKG: []string{"15", "20"},
	}
	tests := map[string]string{
		"en": "2 bags - 15 kg + 20 kg",
		"ro": "2 bagaje - 15 kg + 20 kg",
		"ru": "2 багажа - 15 кг + 20 кг",
	}
	for locale, expected := range tests {
		if got := formatLuggageDescriptor(locale, parsed); got != expected {
			t.Fatalf("expected %q for locale %s, got %q", expected, locale, got)
		}
	}
}

func TestFormatLuggageDescriptorUsesSingularLabels(t *testing.T) {
	parsed := parsedLuggageOption{
		Quantity:      1,
		WeightPartsKG: []string{"20"},
	}
	tests := map[string]string{
		"en": "1 bags - 20 kg",
		"ro": "1 bagaj - 20 kg",
		"ru": "1 багаж - 20 кг",
	}
	for locale, expected := range tests {
		if got := formatLuggageDescriptor(locale, parsed); got != expected {
			t.Fatalf("expected %q for locale %s, got %q", expected, locale, got)
		}
	}
}

func TestSearchCachesRawTravelfusionFlightsForCalendar(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cache := &fakeCalendarCache{}
	service := NewServiceWithDependencies(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID: "RID",
		OutwardFlights: []travelfusion.Flight{
			tfFlight("OUT1", "KIV", "OTP", departure.AddDate(0, 0, -1), 100),
			tfFlight("OUT2", "KIV", "OTP", departure, 200),
		},
	}}, nil, cache, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	_, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        departure,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(cache.calls) != 1 {
		t.Fatalf("expected one calendar cache call, got %d", len(cache.calls))
	}
	if len(cache.calls[0].flights) != 2 {
		t.Fatalf("expected raw TF flights to be cached before date filtering, got %d", len(cache.calls[0].flights))
	}
}

func TestSearchAppliesJavaSearchFilters(t *testing.T) {
	departure := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	matching := tfFlight("OUT1", "KIV", "OTP", departure, 150)
	matching.Segments = []travelfusion.Segment{
		{
			Origin:          "KIV",
			Destination:     "VIE",
			DepartureTime:   departure,
			ArrivalTime:     departure.Add(90 * time.Minute),
			DurationMinutes: 90,
			FlightNumber:    "TF100",
			TravelClass:     "Economy",
		},
		{
			Origin:          "VIE",
			Destination:     "OTP",
			DepartureTime:   departure.Add(150 * time.Minute),
			ArrivalTime:     departure.Add(240 * time.Minute),
			DurationMinutes: 90,
			FlightNumber:    "TF200",
			TravelClass:     "Economy",
		},
	}
	tooExpensive := tfFlight("OUT2", "KIV", "OTP", departure, 500)
	service := NewService(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID:      "RID",
		OutwardFlights: []travelfusion.Flight{matching, tooExpensive},
	}}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode:                "KIV",
		ArrivalAirportCode:                  "OTP",
		DepartureDate:                       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AdultCount:                          1,
		MinPrice:                            float64Ptr(100),
		MaxPrice:                            float64Ptr(200),
		MinSegments:                         intTestPtr(2),
		MaxSegments:                         intTestPtr(2),
		MinTotalDurationMinutes:             intTestPtr(240),
		MaxTotalDurationMinutes:             intTestPtr(240),
		MinIndividualSegmentDurationMinutes: intTestPtr(60),
		MaxIndividualSegmentDurationMinutes: intTestPtr(100),
		MinLayoverMinutes:                   intTestPtr(60),
		MaxLayoverMinutes:                   intTestPtr(60),
		DepartureOutboundFrom:               clockPtr("07:00"),
		DepartureOutboundTo:                 clockPtr("09:00"),
		ArrivalOutboundFrom:                 clockPtr("11:00"),
		ArrivalOutboundTo:                   clockPtr("13:00"),
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(resp.Offers) != 1 || resp.Offers[0].OfferID != "TF-OUT1" {
		t.Fatalf("expected only matching filtered offer, got %+v", resp.Offers)
	}
}

func TestSearchValidatesFilterRanges(t *testing.T) {
	service := NewService(fakeTFClient{err: errors.New("should not be called")}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	_, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AdultCount:           1,
		MinPrice:             float64Ptr(200),
		MaxPrice:             float64Ptr(100),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestSearchKeepsFlightWhenAnySegmentMatchesDepartureDate(t *testing.T) {
	departure := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	firstSegmentDeparture := time.Date(2026, 7, 1, 21, 30, 0, 0, time.UTC)
	secondSegmentDeparture := time.Date(2026, 7, 2, 7, 30, 0, 0, time.UTC)
	flight := tfFlight("OUT1", "BKK", "HKG", firstSegmentDeparture, 100)
	flight.Segments = append(flight.Segments, travelfusion.Segment{
		Origin:          "HKT",
		Destination:     "USM",
		DepartureTime:   secondSegmentDeparture,
		ArrivalTime:     secondSegmentDeparture.Add(55 * time.Minute),
		DurationMinutes: 55,
		FlightNumber:    "PG408",
		TravelClass:     "Economy",
	})
	service := NewService(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID:      "RID",
		OutwardFlights: []travelfusion.Flight{flight},
	}}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "BKK",
		ArrivalAirportCode:   "HKG",
		DepartureDate:        departure,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(resp.Offers) != 1 {
		t.Fatalf("expected overnight itinerary to be kept, got %d offers", len(resp.Offers))
	}
}

func TestSearchFiltersByCalendarDateIgnoringTimezoneOffset(t *testing.T) {
	requestDate := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	tfLocation := time.FixedZone("TF", 3*60*60)
	tfDeparture := time.Date(2026, 7, 2, 0, 0, 0, 0, tfLocation)
	service := NewService(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID:      "RID",
		OutwardFlights: []travelfusion.Flight{tfFlight("OUT1", "BKK", "HKG", tfDeparture, 100)},
	}}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "BKK",
		ArrivalAirportCode:   "HKG",
		DepartureDate:        requestDate,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(resp.Offers) != 1 {
		t.Fatalf("expected timezone-offset date to match calendar date, got %d offers", len(resp.Offers))
	}
}

func TestSearchRoundTripBuildsCheapestPair(t *testing.T) {
	departure := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	returnDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	service := NewService(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID: "RID",
		OutwardFlights: []travelfusion.Flight{
			tfFlight("OUT1", "KIV", "OTP", departure, 200),
			tfFlight("OUT2", "KIV", "OTP", departure, 100),
		},
		ReturnFlights: []travelfusion.Flight{
			tfFlight("RET1", "OTP", "KIV", returnDate, 300),
			tfFlight("RET2", "OTP", "KIV", returnDate, 50),
		},
	}}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	resp, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "KIV",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        departure,
		ReturnDate:           &returnDate,
		AdultCount:           1,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(resp.Offers) != 2 {
		t.Fatalf("expected 2 offers, got %d: %+v", len(resp.Offers), resp.Offers)
	}
	if resp.Offers[0].OfferID != "TF-OUT2-RET2" || resp.Offers[0].Price != 150 {
		t.Fatalf("expected cheapest pair first, got %+v", resp.Offers[0])
	}
}

func TestSearchValidatesRequest(t *testing.T) {
	service := NewService(fakeTFClient{err: errors.New("should not be called")}, nil)
	service.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

	_, err := service.Search(context.Background(), SearchRequest{
		DepartureAirportCode: "kiv",
		ArrivalAirportCode:   "OTP",
		DepartureDate:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		AdultCount:           1,
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func tfFlight(id, origin, destination string, departure time.Time, price float64) travelfusion.Flight {
	arrival := departure.Add(90 * time.Minute)
	return travelfusion.Flight{
		ID:            id,
		Origin:        origin,
		Destination:   destination,
		DepartureTime: departure,
		ArrivalTime:   arrival,
		Price:         price,
		Currency:      "EUR",
		Segments: []travelfusion.Segment{
			{
				Origin:          origin,
				Destination:     destination,
				DepartureTime:   departure,
				ArrivalTime:     arrival,
				DurationMinutes: 90,
				FlightNumber:    "TF100",
				TravelClass:     "Economy",
			},
		},
	}
}

func intTestPtr(value int) *int {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func clockPtr(value string) *time.Time {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		panic(err)
	}
	return &parsed
}
