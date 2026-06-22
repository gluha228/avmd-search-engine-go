package calendar

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid calendar request")
	iataCodePattern   = regexp.MustCompile(`^[A-Z]{3}$`)
)

type PriceStore interface {
	GetMinPrice(ctx context.Context, origin, destination string, date time.Time) (*PriceEntry, error)
	SetMinPriceIfLower(ctx context.Context, origin, destination string, date time.Time, entry PriceEntry) error
}

type CurrencyConverter interface {
	Convert(ctx context.Context, amount float64, from, to string) (float64, error)
}

type Service struct {
	store           PriceStore
	currency        CurrencyConverter
	defaultCurrency string
	logger          *slog.Logger
	now             func() time.Time
}

func NewService(store PriceStore, defaultCurrency string, currency CurrencyConverter, logger *slog.Logger) *Service {
	if defaultCurrency == "" {
		defaultCurrency = "EUR"
	}
	return &Service{
		store:           store,
		currency:        currency,
		defaultCurrency: defaultCurrency,
		logger:          logger,
		now:             time.Now,
	}
}

func (s *Service) GetCalendar(ctx context.Context, req Request) (*Response, error) {
	if err := s.Validate(req); err != nil {
		return nil, err
	}

	days := make([]FlightDay, 0)
	for current := dateOnly(req.DateFrom); !current.After(dateOnly(req.DateTo)); current = current.AddDate(0, 0, 1) {
		entry, err := s.store.GetMinPrice(ctx, req.DepartureAirportCode, req.ArrivalAirportCode, current)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			continue
		}
		currencyCode := entry.CurrencyCode
		if currencyCode == "" {
			currencyCode = s.defaultCurrency
		}
		days = append(days, FlightDay{
			Date:         current.Format(time.DateOnly),
			Price:        entry.Price,
			CurrencyCode: currencyCode,
		})
	}

	return &Response{Calendar: days}, nil
}

func (s *Service) CacheFlights(ctx context.Context, departure, arrival string, flights []travelfusion.Flight) error {
	minByDate := make(map[string]PriceEntry)
	for _, flight := range flights {
		if flight.Price <= 0 || flight.DepartureTime.IsZero() {
			continue
		}
		date := dateOnly(flight.DepartureTime).Format(time.DateOnly)
		price := flight.Price
		if s.currency != nil {
			converted, err := s.currency.Convert(ctx, flight.Price, flight.Currency, s.defaultCurrency)
			if err == nil {
				price = converted
			} else if s.logger != nil {
				s.logger.Warn("failed to convert calendar price, using original price", "from", flight.Currency, "to", s.defaultCurrency, "error", err)
			}
		}
		entry := PriceEntry{
			Price:        price,
			CurrencyCode: s.defaultCurrency,
		}
		if current, ok := minByDate[date]; !ok || entry.Price < current.Price {
			minByDate[date] = entry
		}
	}

	for date, entry := range minByDate {
		parsedDate, err := time.Parse(time.DateOnly, date)
		if err != nil {
			return err
		}
		if err := s.store.SetMinPriceIfLower(ctx, departure, arrival, parsedDate, entry); err != nil {
			return err
		}
	}
	if s.logger != nil && len(minByDate) > 0 {
		s.logger.Debug("cached calendar prices", "departure", departure, "arrival", arrival, "days", len(minByDate))
	}
	return nil
}

func (s *Service) Validate(req Request) error {
	if !iataCodePattern.MatchString(req.DepartureAirportCode) {
		return fmt.Errorf("%w: departureAirportCode must be a 3-letter IATA code", ErrInvalidRequest)
	}
	if !iataCodePattern.MatchString(req.ArrivalAirportCode) {
		return fmt.Errorf("%w: arrivalAirportCode must be a 3-letter IATA code", ErrInvalidRequest)
	}
	if req.DateFrom.IsZero() {
		return fmt.Errorf("%w: dateFrom is required", ErrInvalidRequest)
	}
	if req.DateTo.IsZero() {
		return fmt.Errorf("%w: dateTo is required", ErrInvalidRequest)
	}
	today := dateOnly(s.now())
	if dateOnly(req.DateFrom).Before(today) {
		return fmt.Errorf("%w: dateFrom cannot be in the past", ErrInvalidRequest)
	}
	if dateOnly(req.DateTo).Before(today) {
		return fmt.Errorf("%w: dateTo cannot be in the past", ErrInvalidRequest)
	}
	if dateOnly(req.DateFrom).After(dateOnly(req.DateTo)) {
		return fmt.Errorf("%w: dateFrom cannot be after dateTo", ErrInvalidRequest)
	}
	return nil
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
