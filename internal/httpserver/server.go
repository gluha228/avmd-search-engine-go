package httpserver

import (
	apispec "avmd-search-engine-go/api"
	"avmd-search-engine-go/api/gen"
	"avmd-search-engine-go/internal/calendar"
	"avmd-search-engine-go/internal/config"
	"avmd-search-engine-go/internal/currencies"
	dbstore "avmd-search-engine-go/internal/db"
	"avmd-search-engine-go/internal/flights"
	flightsession "avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/geo"
	"avmd-search-engine-go/internal/redishealth"
	"avmd-search-engine-go/internal/supplierroutes"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
)

type HttpServer struct {
	cfg             *config.Config
	calendarService *calendar.Service
	currencyService *currencies.Service
	flightService   *flights.Service
	geoService      *geo.Service
	routeService    *supplierroutes.Service
	logger          *slog.Logger
	validator       *validator.Validate
}

func NewHttpServer(cfg *config.Config, logger *slog.Logger) *HttpServer {
	return &HttpServer{
		cfg:       cfg,
		logger:    logger,
		validator: validator.New(),
	}
}

func (s *HttpServer) InitHandlers() error {
	tfClient := travelfusion.NewClient(travelfusion.Config{
		BaseURL:             s.cfg.TFBaseURL,
		XmlLoginID:          s.cfg.TFXmlLoginID,
		LoginID:             s.cfg.TFLoginID,
		TimeoutSeconds:      s.cfg.TFTimeoutSeconds,
		PollingAttempts:     s.cfg.TFPollingAttempts,
		PollingDelaySeconds: s.cfg.TFPollingDelaySeconds,
	}, s.logger)
	redishealth.ConfigureLogger(s.logger)
	redisClient := redishealth.NewClient(s.cfg)
	db, err := dbstore.CreateConnection(context.Background(), s.cfg)
	sessionStore := flightsession.NewRedisStore(
		redisClient,
		time.Duration(s.cfg.RedisSessionTTLHours)*time.Hour,
		s.logger,
	)
	priceStore := calendar.NewRedisPriceStore(
		redisClient,
		time.Duration(s.cfg.RedisCalendarTTLHours)*time.Hour,
	)
	currencyStore := currencies.NewRedisStore(redisClient)
	s.currencyService = currencies.NewService(
		tfClient,
		currencyStore,
		currencies.Config{
			UpdateCron: s.cfg.TFCurrenciesUpdateCron,
			UpdateTime: s.cfg.TFCurrenciesUpdateTime,
		},
		s.logger,
	)
	if err := s.currencyService.Start(context.Background()); err != nil && s.logger != nil {
		s.logger.Warn("failed to start currency scheduler", "error", err)
	}
	routeStore := supplierroutes.NewRedisStore(redisClient)
	s.routeService = supplierroutes.NewService(
		tfClient,
		routeStore,
		supplierroutes.Config{
			UpdateCron: s.cfg.TFRoutesUpdateCron,
			UpdateTime: s.cfg.TFRoutesUpdateTime,
		},
		s.logger,
	)
	if err := s.routeService.Start(context.Background()); err != nil && s.logger != nil {
		s.logger.Warn("failed to start TF route scheduler", "error", err)
	}
	if err != nil && s.logger != nil {
		s.logger.Warn("failed to initialize postgres connection", "error", err)
	}
	if err == nil {
		s.geoService = geo.NewServiceWithRouteProvider(geo.NewSQLCRepository(db), s.routeService)
	}
	s.calendarService = calendar.NewService(priceStore, s.cfg.DefaultCurrencyCode, s.currencyService, s.logger)
	s.flightService = flights.NewServiceWithBookingDependencies(
		tfClient,
		sessionStore,
		s.calendarService,
		s.currencyService,
		s.cfg.DefaultCurrencyCode,
		s.logger,
	)
	return nil
}

func (s *HttpServer) CreateHandler() http.Handler {
	r := chi.NewRouter()
	strictServer := api.NewStrictHandler(s, nil)
	r.Use(middleware.Logger)
	r.Use(withCORS(s.cfg.CORSAllowedOrigins))
	r.Use(withServerErrorLogging(s.logger))
	r.Use(withRequestContext)
	r.Get("/v3/api-docs", serveOpenAPISpec)
	r.Get("/swagger-ui", redirectToSwaggerUI)
	r.Get("/swagger-ui/", redirectToSwaggerUI)
	r.Get("/swagger-ui/index.html", serveSwaggerUI)
	return api.HandlerFromMux(strictServer, r)
}

type requestContextKey struct{}

func withRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestContextKey{}, r)))
	})
}

func serveOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	spec := strings.Replace(string(apispec.OpenAPIYAML), "http://localhost:8080", requestOrigin(r), 1)
	_, _ = w.Write([]byte(spec))
}

func requestOrigin(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

func redirectToSwaggerUI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/swagger-ui/index.html", http.StatusFound)
}

func serveSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIHTML))
}

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AVMD Search Engine Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/v3/api-docs",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`
