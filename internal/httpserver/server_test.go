package httpserver

import (
	"avmd-search-engine-go/internal/config"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
