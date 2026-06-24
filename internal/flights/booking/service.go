package booking

import (
	"avmd-search-engine-go/internal/flights"
	"avmd-search-engine-go/internal/flights/session"
	"context"
	"log/slog"
)

var (
	ErrInvalidRequest = flights.ErrInvalidRequest
	ErrNotFound       = flights.ErrNotFound
)

type TravelfusionClient = flights.TravelfusionClient
type SessionStore = flights.SessionStore
type CalendarCache = flights.CalendarCache
type CurrencyConverter = flights.CurrencyConverter
type FlightAirportLookup = flights.FlightAirportLookup

type PassengerDataRequest = session.PassengerDataRequest
type PassengerDataResponse = session.PassengerDataResponse
type SelectedOffer = session.SelectedOffer
type EnrichedOffer = session.EnrichedOffer
type SegmentSeatMap = session.SegmentSeatMap
type Offer = session.Offer

type Service struct {
	inner *flights.Service
}

func NewService(
	tfClient TravelfusionClient,
	sessionStore SessionStore,
	currencyConverter CurrencyConverter,
	airportLookup FlightAirportLookup,
	defaultCurrency string,
	logger *slog.Logger,
) *Service {
	return &Service{inner: flights.NewServiceWithAirportLookup(
		tfClient,
		sessionStore,
		nil,
		currencyConverter,
		airportLookup,
		defaultCurrency,
		logger,
	)}
}

func WithLocale(ctx context.Context, locale string) context.Context {
	return flights.WithLocale(ctx, locale)
}

func (s *Service) SetOperatorLogoURLPattern(pattern string) {
	s.inner.SetOperatorLogoURLPattern(pattern)
}

func (s *Service) ProcessPassengerData(ctx context.Context, req PassengerDataRequest) (*PassengerDataResponse, error) {
	return s.inner.ProcessPassengerData(ctx, req)
}

func (s *Service) GetSelectedOffer(ctx context.Context, searchID, offerID string) (*SelectedOffer, error) {
	return s.inner.GetSelectedOffer(ctx, searchID, offerID)
}

func (s *Service) EnrichOffer(ctx context.Context, offer Offer, locale string) (EnrichedOffer, error) {
	return s.inner.EnrichOffer(ctx, offer, locale)
}

func (s *Service) GetSeatMap(ctx context.Context, searchID, offerID string) ([]SegmentSeatMap, error) {
	return s.inner.GetSeatMap(ctx, searchID, offerID)
}
