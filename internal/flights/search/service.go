package search

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

var ErrInvalidRequest = errors.New("invalid flight search request")

var iataCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

const defaultOperatorLogoURLPattern = "https://www.travelfusion.com/images/operators/p%s.gif"

type TravelfusionClient interface {
	SearchStream(ctx context.Context, req travelfusion.SearchRequest) <-chan travelfusion.SearchUpdate
}

type SessionStore interface {
	Create(ctx context.Context, session session.FlightSearchSession) (string, error)
	Save(ctx context.Context, searchID string, session session.FlightSearchSession) error
}

type CalendarCache interface {
	CacheFlights(ctx context.Context, departure, arrival string, flights []travelfusion.Flight) error
}

type CurrencyConverter interface {
	Convert(ctx context.Context, amount float64, from, to string) (float64, error)
}

type FlightAirportLookup interface {
	FlightAirportsByIATACodes(ctx context.Context, codes []string, locale string) (map[string]session.FlightAirport, error)
}

type Service struct {
	tfClient               TravelfusionClient
	sessionStore           SessionStore
	calendar               CalendarCache
	currency               CurrencyConverter
	airportLookup          FlightAirportLookup
	defaultCurrency        string
	operatorLogoURLPattern string
	logger                 *slog.Logger
	now                    func() time.Time
}

func NewService(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	calendarCache CalendarCache,
	currencyConverter CurrencyConverter,
	airportLookup FlightAirportLookup,
	defaultCurrency string,
	logger *slog.Logger,
) *Service {
	return &Service{
		tfClient:               tfClient,
		sessionStore:           sessionStore,
		calendar:               calendarCache,
		currency:               currencyConverter,
		airportLookup:          airportLookup,
		defaultCurrency:        strings.ToUpper(strings.TrimSpace(defaultCurrency)),
		operatorLogoURLPattern: defaultOperatorLogoURLPattern,
		logger:                 logger,
		now:                    time.Now,
	}
}

func (s *Service) SetOperatorLogoURLPattern(pattern string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = defaultOperatorLogoURLPattern
	}
	s.operatorLogoURLPattern = pattern
}
