package httpserver

import (
	"avmd-search-engine-go/internal/calendar"
	"avmd-search-engine-go/internal/config"
	"avmd-search-engine-go/internal/flights"
	"avmd-search-engine-go/internal/testsupport"
	"avmd-search-engine-go/internal/travelfusion"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTFClient struct {
	result         *travelfusion.SearchResult
	searchUpdates  []travelfusion.SearchUpdate
	processDetails *travelfusion.ProcessDetailsResult
	processTerms   *travelfusion.ProcessTermsResult
	err            error
}

func (f fakeTFClient) Search(context.Context, travelfusion.SearchRequest) (*travelfusion.SearchResult, error) {
	return f.result, f.err
}

func (f fakeTFClient) SearchStream(ctx context.Context, _ travelfusion.SearchRequest) <-chan travelfusion.SearchUpdate {
	return testsupport.SearchUpdateStream(ctx, f.result, f.searchUpdates, f.err)
}

func (f fakeTFClient) ProcessDetails(context.Context, travelfusion.ProcessDetailsRequest) (*travelfusion.ProcessDetailsResult, error) {
	return f.processDetails, f.err
}

func (f fakeTFClient) ProcessTerms(context.Context, travelfusion.ProcessTermsRequest) (*travelfusion.ProcessTermsResult, error) {
	return f.processTerms, f.err
}

type fakeSessionStore struct {
	searchID string
	session  flights.FlightSearchSession
	err      error
}

type fakeCalendarPriceStore struct {
	entries map[string]calendar.PriceEntry
	err     error
}

type fakeFlightAirportLookup struct {
	locale   string
	airports map[string]flights.FlightAirport
}

func (f *fakeSessionStore) Create(_ context.Context, session flights.FlightSearchSession) (string, error) {
	f.session = session
	return f.searchID, f.err
}

func (f *fakeSessionStore) Save(_ context.Context, _ string, session flights.FlightSearchSession) error {
	f.session = session
	return f.err
}

func (f *fakeSessionStore) Get(_ context.Context, _ string) (*flights.FlightSearchSession, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.session, nil
}

func (f *fakeCalendarPriceStore) GetMinPrice(_ context.Context, origin, destination string, date time.Time) (*calendar.PriceEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	entry, ok := f.entries[origin+":"+destination+":"+date.Format(time.DateOnly)]
	if !ok {
		return nil, nil
	}
	return &entry, nil
}

func (f *fakeCalendarPriceStore) SetMinPriceIfLower(_ context.Context, origin, destination string, date time.Time, entry calendar.PriceEntry) error {
	f.entries[origin+":"+destination+":"+date.Format(time.DateOnly)] = entry
	return nil
}

func (f *fakeFlightAirportLookup) FlightAirportsByIATACodes(_ context.Context, _ []string, locale string) (map[string]flights.FlightAirport, error) {
	f.locale = locale
	return f.airports, nil
}

func TestOpenAPISpecEndpoint(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/v3/api-docs", nil)
	request.Host = "api.example.test"
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/yaml") {
		t.Fatalf("expected yaml content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "openapi: 3.0.0") {
		t.Fatalf("expected OpenAPI document, got %q", body)
	}
	if !strings.Contains(body, "url: http://api.example.test") {
		t.Fatalf("expected OpenAPI server url to use request host, got %q", body)
	}
}

func TestOpenAPISpecEndpointUsesForwardedHost(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/v3/api-docs", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "public.example.test")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "url: https://public.example.test") {
		t.Fatalf("expected OpenAPI server url to use forwarded origin, got %q", body)
	}
}

func TestSwaggerUIEndpoint(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/swagger-ui/index.html", nil)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") || !strings.Contains(body, `url: "/v3/api-docs"`) {
		t.Fatalf("expected Swagger UI HTML wired to /v3/api-docs, got %q", body)
	}
}

func TestSwaggerUIRedirect(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/swagger-ui", nil)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/swagger-ui/index.html" {
		t.Fatalf("expected redirect to /swagger-ui/index.html, got %q", location)
	}
}

func TestCORSPreflightAllowsConfiguredLocalFrontend(t *testing.T) {
	server := NewHttpServer(&config.Config{
		CORSAllowedOrigins: "http://localhost:5173",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/flights/search", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type, accept-language")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost origin to be allowed, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("expected POST to be allowed, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Accept-Language") {
		t.Fatalf("expected Accept-Language to be allowed, got %q", got)
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	server := NewHttpServer(&config.Config{
		CORSAllowedOrigins: "http://localhost:5173",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/flights/search", nil)
	request.Header.Set("Origin", "http://evil.example")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected unknown origin not to be allowed, got %q", got)
	}
}

func TestSearchFlightsStreamsSSE(t *testing.T) {
	tfLocation := time.FixedZone("TF", 3*60*60)
	departure := time.Date(2026, 7, 2, 22, 30, 0, 0, tfLocation)
	segmentArrival := departure.Add(90 * time.Minute)
	store := &fakeSessionStore{searchID: "search-1"}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID: "RID",
		OutwardFlights: []travelfusion.Flight{
			{
				ID:            "OUT1",
				Origin:        "KIV",
				Destination:   "OTP",
				DepartureTime: departure,
				ArrivalTime:   segmentArrival,
				Price:         100,
				Currency:      "EUR",
				PassengerPrices: travelfusion.PassengerPrices{
					Adults: []float64{100},
				},
				Segments: []travelfusion.Segment{
					{
						Origin:          "KIV",
						Destination:     "OTP",
						DepartureTime:   departure,
						ArrivalTime:     segmentArrival,
						DurationMinutes: 90,
						FlightNumber:    "TF100",
						TravelClass:     "Economy",
					},
				},
			},
		},
	}}, store, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/flights/search?departureAirportCode=KIV&arrivalAirportCode=OTP&departureDate=2026-07-02&adultCount=1",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", contentType)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"event: search_id", `"search_id":"search-1"`, "event: offers", `"offer_id":"TF-OUT1"`, `"passenger_prices":{"adults":[100],"children":[],"infants":[]}`, `"departure_time":"2026-07-02T22:30:00"`, "event: done\ndata: \n\n"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected SSE body to contain %q, got %q", expected, body)
		}
	}
	if strings.Contains(body, "+03:00") {
		t.Fatalf("expected SSE segment times without timezone, got %q", body)
	}
	if strings.Contains(body, "travel_class") {
		t.Fatalf("expected SSE segments not to expose travel_class, got %q", body)
	}
	if store.session.TFRoutingID != "RID" || len(store.session.TFOffers) != 1 {
		t.Fatalf("expected final session to be saved, got %+v", store.session)
	}
}

func TestSearchFlightsStreamsLocalizedAirportObjects(t *testing.T) {
	departure := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{searchID: "search-1"}
	lookup := &fakeFlightAirportLookup{airports: map[string]flights.FlightAirport{
		"KIV": {Code: "KIV", CityName: "Кишинев"},
		"OTP": {Code: "OTP", CityName: "Бухарест"},
	}}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithAirportLookup(fakeTFClient{result: &travelfusion.SearchResult{
		RoutingID: "RID",
		OutwardFlights: []travelfusion.Flight{
			{
				ID:            "OUT1",
				Origin:        "KIV",
				Destination:   "OTP",
				DepartureTime: departure,
				ArrivalTime:   departure.Add(90 * time.Minute),
				Price:         100,
				Currency:      "EUR",
				Segments: []travelfusion.Segment{{
					Origin:        "KIV",
					Destination:   "OTP",
					DepartureTime: departure,
					ArrivalTime:   departure.Add(90 * time.Minute),
				}},
			},
		},
	}}, store, nil, nil, lookup, "EUR", nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/flights/search?departureAirportCode=KIV&arrivalAirportCode=OTP&departureDate=2026-07-02&adultCount=1",
		nil,
	)
	request.Header.Set("Accept-Language", "ru")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{`"departure_flight_airport":{"code":"KIV","city_name":"Кишинев"}`, `"arrival_flight_airport":{"code":"OTP","city_name":"Бухарест"}`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected SSE body to contain %q, got %s", expected, body)
		}
	}
	if lookup.locale != "ru" {
		t.Fatalf("expected Accept-Language to be used for airport lookup, got %q", lookup.locale)
	}
	if store.session.TFOffers[0].OutboundFlight.DepartureAirportCode != "KIV" {
		t.Fatalf("expected cached offer to keep airport codes, got %+v", store.session.TFOffers[0])
	}
}

func TestSearchFlightsStreamsOffersAsTheyArrive(t *testing.T) {
	departure := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	store := &fakeSessionStore{searchID: "search-1"}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{searchUpdates: []travelfusion.SearchUpdate{
		{RoutingID: "RID"},
		{RoutingID: "RID", OutwardFlights: []travelfusion.Flight{
			{
				ID:            "OUT1",
				Origin:        "KIV",
				Destination:   "OTP",
				DepartureTime: departure,
				ArrivalTime:   departure.Add(90 * time.Minute),
				Price:         100,
				Currency:      "EUR",
				Segments: []travelfusion.Segment{
					{Origin: "KIV", Destination: "OTP", DepartureTime: departure, ArrivalTime: departure.Add(90 * time.Minute), DurationMinutes: 90},
				},
			},
		}},
		{RoutingID: "RID", OutwardFlights: []travelfusion.Flight{
			{
				ID:            "OUT2",
				Origin:        "KIV",
				Destination:   "OTP",
				DepartureTime: departure,
				ArrivalTime:   departure.Add(120 * time.Minute),
				Price:         200,
				Currency:      "EUR",
				Segments: []travelfusion.Segment{
					{Origin: "KIV", Destination: "OTP", DepartureTime: departure, ArrivalTime: departure.Add(120 * time.Minute), DurationMinutes: 120},
				},
			},
		}},
	}}, store, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/flights/search?departureAirportCode=KIV&arrivalAirportCode=OTP&departureDate=2026-07-02&adultCount=1",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if count := strings.Count(body, "event: offers"); count != 2 {
		t.Fatalf("expected two offers events, got %d: %s", count, body)
	}
	if !strings.Contains(body, `"offer_id":"TF-OUT1"`) || !strings.Contains(body, `"offer_id":"TF-OUT2"`) {
		t.Fatalf("expected both streamed offers, got %s", body)
	}
	if len(store.session.TFOffers) != 2 {
		t.Fatalf("expected final session to contain two offers, got %+v", store.session)
	}
}

func TestGetSelectedOfferReturnsCachedSessionOffer(t *testing.T) {
	departure := time.Date(2026, 7, 2, 8, 30, 0, 0, time.UTC)
	arrival := departure.Add(90 * time.Minute)
	store := &fakeSessionStore{session: flights.FlightSearchSession{
		Params: flights.SearchRequest{
			DepartureAirportCode: "KIV",
			ArrivalAirportCode:   "OTP",
			DepartureDate:        departure,
			AdultCount:           1,
		},
		TFRoutingID: "RID",
		TFOffers: []flights.Offer{
			{
				OfferID:      "TF-OUT1",
				CurrencyCode: "EUR",
				Price:        100,
				PassengerPrices: flights.PassengerPrices{
					Adults: []float64{100},
				},
				OutboundFlight: flights.Flight{
					DepartureAirportCode: "KIV",
					ArrivalAirportCode:   "OTP",
					SeatsAvailable:       4,
					Price:                100,
					Segments: []flights.Segment{
						{
							SegmentID:            1,
							DepartureAirportCode: "KIV",
							ArrivalAirportCode:   "OTP",
							DepartureTime:        &departure,
							ArrivalTime:          &arrival,
							DurationMinutes:      90,
							FlightNumber:         "TF100",
							TravelClass:          "Economy",
						},
					},
				},
			},
		},
	}}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	isOptional := false
	perPassenger := true
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{processDetails: &travelfusion.ProcessDetailsResult{
		RoutingID: "RID",
		RequiredParameters: []travelfusion.RequiredParameter{
			{
				Name:         "PassportNumber",
				Type:         "text",
				DisplayText:  "Passport number",
				IsOptional:   &isOptional,
				PerPassenger: &perPassenger,
			},
		},
	}}, store, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/booking/selected-offer?searchId=search-1&offerId=TF-OUT1", nil)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	offer := response["offer"].(map[string]any)
	searchParams := response["search_params"].(map[string]any)
	if offer["offer_id"] != "TF-OUT1" || offer["price"] != float64(100) {
		t.Fatalf("unexpected offer response: %s", recorder.Body.String())
	}
	passengerPrices := offer["passenger_prices"].(map[string]any)
	if adults := passengerPrices["adults"].([]any); len(adults) != 1 || adults[0] != float64(100) {
		t.Fatalf("unexpected passenger_prices response: %s", recorder.Body.String())
	}
	outboundFlight := offer["outbound_flight"].(map[string]any)
	if _, ok := outboundFlight["price"]; ok {
		t.Fatalf("expected outbound_flight price to be omitted, got %s", recorder.Body.String())
	}
	if searchParams["departure_airport_code"] != "KIV" || searchParams["adult_count"] != float64(1) {
		t.Fatalf("unexpected search params response: %s", recorder.Body.String())
	}
	if _, ok := response["available_ancillaries"]; ok {
		t.Fatalf("expected available_ancillaries to be omitted, got %s", recorder.Body.String())
	}
	additionalFields, ok := response["additional_fields"].([]any)
	if !ok || len(additionalFields) != 1 {
		t.Fatalf("expected additional_fields array, got %s", recorder.Body.String())
	}
	field := additionalFields[0].(map[string]any)
	if field["code"] != "PASSPORT_NUMBER" || field["input_type"] != "TEXT" {
		t.Fatalf("unexpected additional_fields response: %s", recorder.Body.String())
	}
}

func TestGetSelectedOfferLocalizesLuggageAdditionalFields(t *testing.T) {
	departure := time.Date(2026, 7, 2, 8, 30, 0, 0, time.UTC)
	store := &fakeSessionStore{session: flights.FlightSearchSession{
		Params: flights.SearchRequest{
			DepartureAirportCode: "KIV",
			ArrivalAirportCode:   "OTP",
			DepartureDate:        departure,
			AdultCount:           1,
		},
		TFRoutingID: "RID",
		TFOffers: []flights.Offer{
			{OfferID: "TF-OUT1", CurrencyCode: "EUR", Price: 100},
		},
	}}
	isOptional := false
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{processDetails: &travelfusion.ProcessDetailsResult{
		RoutingID: "RID",
		RequiredParameters: []travelfusion.RequiredParameter{
			{
				Name:        "LuggageOptions",
				Type:        "value_select",
				DisplayText: "LuggageOptions: 1 (1 bags - 20Kg total - 25.00 EUR)",
				IsOptional:  &isOptional,
			},
		},
	}}, store, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/booking/selected-offer?searchId=search-1&offerId=TF-OUT1", nil)
	request.Header.Set("Accept-Language", "ru")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"label":"1 багаж - 20 кг"`) {
		t.Fatalf("expected localized luggage label, got %s", recorder.Body.String())
	}
}

func TestGetSeatMapReturnsCachedSeats(t *testing.T) {
	price := 20.0
	currency := "EUR"
	description := "Exit row"
	store := &fakeSessionStore{session: flights.FlightSearchSession{
		TFSeatMapByOfferID: map[string][]flights.SegmentSeatMap{
			"TF-OUT1": {
				{
					SegmentID:    1,
					Origin:       "KIV",
					Destination:  "OTP",
					FlightNumber: "TF100",
					Seats: []flights.SeatDetail{
						{
							Code:            "12A",
							Type:            flights.SeatTypeExitRow,
							SeatDescription: &description,
							Price:           &price,
							CurrencyCode:    &currency,
							Row:             12,
							Col:             0,
							IsAvailable:     true,
						},
					},
				},
			},
		},
	}}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{}, store, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/booking/seats?searchId=search-1&offerId=TF-OUT1", nil)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	for _, expected := range []string{`"segment_id":1`, `"code":"12A"`, `"type":"EXIT_ROW"`, `"price":20`, `"currency_code":"EUR"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("expected response to contain %q, got %s", expected, recorder.Body.String())
		}
	}
}

func TestSubmitPassengerDataReturnsProcessTermsResponse(t *testing.T) {
	finalAmount := 222.5
	sessionStore := &fakeSessionStore{session: flights.FlightSearchSession{
		Params:          flights.SearchRequest{AdultCount: 1},
		TFRoutingID:     "RID",
		SelectedOfferID: "TF-OUT1",
		TFOffers:        []flights.Offer{{OfferID: "TF-OUT1"}},
	}}
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server.flightService = flights.NewServiceWithSessionStore(fakeTFClient{
		processTerms: &travelfusion.ProcessTermsResult{
			RoutingID:          "RID",
			TFBookingReference: "BOOK123",
			FinalAmount:        &finalAmount,
			FinalCurrency:      "EUR",
			SupplierResponses: []travelfusion.SupplierResponse{
				{Name: "prebooking", Type: "html", Data: "<p>ok</p>"},
			},
		},
	}, sessionStore, nil)

	body := `{
		"search_id":"search-1",
		"offer_id":"TF-OUT1",
		"passengers":[{
			"title":"Mr",
			"first_name":"John",
			"last_name":"Doe",
			"date_of_birth":"1990-05-10",
			"citizenship_country_code":"US"
		}],
		"contact_data":{
			"email":"john@example.com",
			"phone":{"international_code":"+373","number":"69123456"}
		}
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/booking/passenger-data", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["tf_booking_reference"] != "BOOK123" || response["final_currency"] != "EUR" {
		t.Fatalf("unexpected response: %v", response)
	}
	supplierResponses, ok := response["supplier_responses"].([]any)
	if !ok || len(supplierResponses) != 1 {
		t.Fatalf("expected supplier response, got %v", response["supplier_responses"])
	}
}

func TestGetCalendarReturnsCachedPrices(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	calendarService := calendar.NewService(&fakeCalendarPriceStore{entries: map[string]calendar.PriceEntry{
		"KIV:OTP:2026-07-02": {Price: 123.45, CurrencyCode: "EUR"},
	}}, "EUR", nil, nil)
	server.calendarService = calendarService

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/flights/calendar?departureAirportCode=KIV&arrivalAirportCode=OTP&dateFrom=2026-07-01&dateTo=2026-07-03",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"date":"2026-07-02"`) || !strings.Contains(body, `"price":123.45`) {
		t.Fatalf("unexpected calendar response: %s", body)
	}
}

func TestServerLogsUnexpectedFiveHundreds(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
	server := NewHttpServer(&config.Config{}, logger)
	calendarService := calendar.NewService(&fakeCalendarPriceStore{err: errors.New("redis unavailable")}, "EUR", nil, nil)
	server.calendarService = calendarService

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/flights/calendar?departureAirportCode=BKK&arrivalAirportCode=HKG&dateFrom=2026-07-01&dateTo=2026-07-31",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.CreateHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "request completed with unexpected server error") ||
		!strings.Contains(logOutput, "/api/v1/flights/calendar") ||
		!strings.Contains(logOutput, "redis unavailable") {
		t.Fatalf("expected 500 details to be logged, got %q", logOutput)
	}
}

func TestServerLogsUnexpectedPanics(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
	handler := withServerErrorLogging(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "unexpected panic while handling request") ||
		!strings.Contains(logOutput, "boom") ||
		!strings.Contains(logOutput, "request completed with unexpected server error") {
		t.Fatalf("expected panic details to be logged, got %q", logOutput)
	}
}
