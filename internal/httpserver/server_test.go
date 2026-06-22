package httpserver

import (
	"avmd-search-engine-go/internal/calendar"
	"avmd-search-engine-go/internal/config"
	"avmd-search-engine-go/internal/flights"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeTFClient struct {
	result *travelfusion.SearchResult
	err    error
}

func (f fakeTFClient) Search(context.Context, travelfusion.SearchRequest) (*travelfusion.SearchResult, error) {
	return f.result, f.err
}

type fakeSessionStore struct {
	searchID string
	session  flights.FlightSearchSession
	err      error
}

type fakeCalendarPriceStore struct {
	entries map[string]calendar.PriceEntry
}

func (f *fakeSessionStore) Create(_ context.Context, session flights.FlightSearchSession) (string, error) {
	f.session = session
	return f.searchID, f.err
}

func (f *fakeSessionStore) Save(_ context.Context, _ string, session flights.FlightSearchSession) error {
	f.session = session
	return f.err
}

func (f *fakeCalendarPriceStore) GetMinPrice(_ context.Context, origin, destination string, date time.Time) (*calendar.PriceEntry, error) {
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

func TestSearchFlightsStreamsSSE(t *testing.T) {
	departure := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
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
	for _, expected := range []string{"event: search_id", `"search_id":"search-1"`, "event: offers", `"offer_id":"OUT1"`, "event: done\ndata: \n\n"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected SSE body to contain %q, got %q", expected, body)
		}
	}
	if store.session.TFRoutingID != "RID" || len(store.session.TFOffers) != 1 {
		t.Fatalf("expected final session to be saved, got %+v", store.session)
	}
}

func TestGetCalendarReturnsCachedPrices(t *testing.T) {
	server := NewHttpServer(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	calendarService := calendar.NewService(&fakeCalendarPriceStore{entries: map[string]calendar.PriceEntry{
		"KIV:OTP:2026-07-02": {Price: 123.45, CurrencyCode: "EUR"},
	}}, nil)
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
