package currencies

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var ErrRateUnavailable = errors.New("currency rate unavailable")

type TravelfusionClient interface {
	GetCurrencies(ctx context.Context) (map[string]travelfusion.Currency, error)
}

type Store interface {
	GetRate(ctx context.Context, currencyCode string) (float64, error)
	SetRate(ctx context.Context, currencyCode string, rate float64) error
	LastUpdate(ctx context.Context) (*time.Time, error)
	SetLastUpdate(ctx context.Context, t time.Time) error
}

type Config struct {
	UpdateCron string
	UpdateTime string
}

type Service struct {
	client    TravelfusionClient
	store     Store
	cfg       Config
	logger    *slog.Logger
	now       func() time.Time
	scheduler *cron.Cron
}

func NewService(client TravelfusionClient, store Store, cfg Config, logger *slog.Logger) *Service {
	if strings.TrimSpace(cfg.UpdateCron) == "" {
		cfg.UpdateCron = "0 0 3 * * ?"
	}
	if strings.TrimSpace(cfg.UpdateTime) == "" {
		cfg.UpdateTime = "03:00"
	}
	return &Service{
		client: client,
		store:  store,
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
	}
}

func (s *Service) Convert(ctx context.Context, amount float64, from, to string) (float64, error) {
	if amount == 0 {
		return 0, nil
	}
	if strings.TrimSpace(from) == "" || strings.EqualFold(from, to) {
		return roundMoney(amount), nil
	}
	rate, err := s.GetRate(ctx, from, to)
	if err != nil {
		return 0, err
	}
	return roundMoney(amount * rate), nil
}

func (s *Service) GetRate(ctx context.Context, fromCurrencyCode, toCurrencyCode string) (float64, error) {
	if strings.TrimSpace(fromCurrencyCode) == "" || strings.TrimSpace(toCurrencyCode) == "" {
		return 0, fmt.Errorf("%w: currency codes cannot be blank", ErrRateUnavailable)
	}
	from := strings.ToUpper(strings.TrimSpace(fromCurrencyCode))
	to := strings.ToUpper(strings.TrimSpace(toCurrencyCode))
	if from == to {
		return 1, nil
	}

	fromUSDRate, err := s.usdRate(ctx, from)
	if err != nil {
		return 0, err
	}
	toUSDRate, err := s.usdRate(ctx, to)
	if err != nil {
		return 0, err
	}
	if fromUSDRate <= 0 || toUSDRate <= 0 {
		return 0, fmt.Errorf("%w: invalid currency rate", ErrRateUnavailable)
	}
	return toUSDRate / fromUSDRate, nil
}

func (s *Service) RefreshIfNeeded(ctx context.Context) error {
	needed, err := s.NeedsRefresh(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("error checking currency cache update status", "error", err)
		}
		needed = true
	}
	if !needed {
		if s.logger != nil {
			s.logger.Info("currency cache is up to date, no update needed")
		}
		return nil
	}
	return s.Refresh(ctx)
}

func (s *Service) Refresh(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("updating currency rates from TravelFusion API")
	}
	currencies, err := s.client.GetCurrencies(ctx)
	if err != nil {
		return fmt.Errorf("get currencies from travelfusion: %w", err)
	}
	if len(currencies) == 0 {
		if s.logger != nil {
			s.logger.Warn("no currencies returned from TravelFusion API, keeping existing cache")
		}
		return nil
	}

	storedCount := 0
	for code, currency := range currencies {
		if currency.USDRate <= 0 {
			continue
		}
		if err := s.store.SetRate(ctx, code, currency.USDRate); err != nil {
			return err
		}
		storedCount++
	}
	if err := s.store.SetRate(ctx, "USD", 1); err != nil {
		return err
	}
	if err := s.store.SetLastUpdate(ctx, s.now()); err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info("successfully updated currency rates in cache", "count", storedCount)
	}
	return nil
}

func (s *Service) NeedsRefresh(ctx context.Context) (bool, error) {
	lastUpdate, err := s.store.LastUpdate(ctx)
	if err != nil {
		return true, err
	}
	if lastUpdate == nil {
		return true, nil
	}
	return lastUpdate.Before(s.latestScheduledUpdateTime()), nil
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.RefreshIfNeeded(ctx); err != nil && s.logger != nil {
		s.logger.Warn("failed to update currency rates from TravelFusion API, keeping existing cache", "error", err)
	}
	scheduler := cron.New(cron.WithSeconds())
	if _, err := scheduler.AddFunc(s.cfg.UpdateCron, func() {
		if s.logger != nil {
			s.logger.Info("scheduled currency cache update triggered")
		}
		if err := s.RefreshIfNeeded(context.Background()); err != nil && s.logger != nil {
			s.logger.Warn("failed to update currency rates from TravelFusion API, keeping existing cache", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("schedule currency update: %w", err)
	}
	s.scheduler = scheduler
	scheduler.Start()
	return nil
}

func (s *Service) Stop() {
	if s.scheduler != nil {
		<-s.scheduler.Stop().Done()
	}
}

func (s *Service) usdRate(ctx context.Context, currencyCode string) (float64, error) {
	if currencyCode == "USD" {
		return 1, nil
	}
	rate, err := s.store.GetRate(ctx, currencyCode)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrRateUnavailable, currencyCode)
	}
	return rate, nil
}

func (s *Service) latestScheduledUpdateTime() time.Time {
	updateTime := strings.TrimSpace(s.cfg.UpdateTime)
	if updateTime == "" {
		updateTime = "03:00"
	}
	parsed, err := time.Parse("15:04", updateTime)
	if err != nil {
		parsed = time.Date(0, 1, 1, 3, 0, 0, 0, time.Local)
	}
	now := s.now()
	todayAtUpdateTime := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if now.Before(todayAtUpdateTime) {
		return todayAtUpdateTime.AddDate(0, 0, -1)
	}
	return todayAtUpdateTime
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
