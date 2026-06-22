package flights

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeTFClient struct {
	result *travelfusion.SearchResult
	err    error
}

type fakeSessionStore struct {
	searchID string
	session  FlightSearchSession
	err      error
}

func (f fakeTFClient) Search(context.Context, travelfusion.SearchRequest) (*travelfusion.SearchResult, error) {
	return f.result, f.err
}

func (f *fakeSessionStore) Create(_ context.Context, session FlightSearchSession) (string, error) {
	f.session = session
	return f.searchID, f.err
}

func (f *fakeSessionStore) Save(_ context.Context, _ string, session FlightSearchSession) error {
	f.session = session
	return f.err
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
	if resp.Offers[0].OfferID != "OUT1" || resp.Offers[0].Price != 100 {
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
	if len(resp.Offers) != 1 || resp.Offers[0].OfferID != "OUT1" {
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
	if resp.Offers[0].OfferID != "OUT2_RET2" || resp.Offers[0].Price != 150 {
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
