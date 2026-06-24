package booking

import (
	"avmd-search-engine-go/internal/flights/session"
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid flight booking request")
	ErrNotFound       = errors.New("flight booking resource not found")
)

const defaultOperatorLogoURLPattern = "https://www.travelfusion.com/images/operators/p%s.gif"

type TravelfusionClient interface {
	ProcessDetails(ctx context.Context, req travelfusion.ProcessDetailsRequest) (*travelfusion.ProcessDetailsResult, error)
	ProcessTerms(ctx context.Context, req travelfusion.ProcessTermsRequest) (*travelfusion.ProcessTermsResult, error)
}

type SessionStore interface {
	Save(ctx context.Context, searchID string, session session.FlightSearchSession) error
	Get(ctx context.Context, searchID string) (*session.FlightSearchSession, error)
}

type CurrencyConverter interface {
	Convert(ctx context.Context, amount float64, from, to string) (float64, error)
}

type FlightAirportLookup interface {
	FlightAirportsByIATACodes(ctx context.Context, codes []string, locale string) (map[string]session.FlightAirport, error)
}

type ContactDetailsSink interface {
	AppendContactDetails(ctx context.Context, details session.ContactData, createdAt time.Time) error
}

type PassengerDataRequest = session.PassengerDataRequest
type PassengerDataResponse = session.PassengerDataResponse
type SelectedOffer = session.SelectedOffer
type EnrichedOffer = session.EnrichedOffer
type SegmentSeatMap = session.SegmentSeatMap
type Offer = session.Offer
type ContactData = session.ContactData

type Service struct {
	tfClient               TravelfusionClient
	sessionStore           SessionStore
	contactDetailsSink     ContactDetailsSink
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
	currencyConverter CurrencyConverter,
	airportLookup FlightAirportLookup,
	defaultCurrency string,
	logger *slog.Logger,
) *Service {
	return &Service{
		tfClient:               tfClient,
		sessionStore:           sessionStore,
		currency:               currencyConverter,
		airportLookup:          airportLookup,
		defaultCurrency:        strings.ToUpper(strings.TrimSpace(defaultCurrency)),
		operatorLogoURLPattern: defaultOperatorLogoURLPattern,
		logger:                 logger,
		now:                    time.Now,
	}
}

func (s *Service) SetContactDetailsSink(sink ContactDetailsSink) {
	s.contactDetailsSink = sink
}

type localeContextKey struct{}

func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey{}, normalizeLocale(locale))
}

func (s *Service) SetOperatorLogoURLPattern(pattern string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		pattern = defaultOperatorLogoURLPattern
	}
	s.operatorLogoURLPattern = pattern
}
