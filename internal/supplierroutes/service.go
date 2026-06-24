package supplierroutes

import (
	"avmd-search-engine-go/internal/travelfusion"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type TravelfusionClient interface {
	GetBranchSupplierList(ctx context.Context) ([]string, error)
	ListSupplierRoutes(ctx context.Context, supplier string, oneWayOnlyAirportRoutes bool) (*travelfusion.SupplierRoutesResult, error)
}

type Store interface {
	IsValidAirportRoute(ctx context.Context, originCode, destinationCode string) (bool, error)
	IsKnownAirport(ctx context.Context, airportCode string) (bool, error)
	IsValidCityRoute(ctx context.Context, originCode, destinationCode string) (bool, error)
	ReplaceRoutes(ctx context.Context, airportRoutes, cityRoutes, knownAirports []string) error
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
		cfg.UpdateCron = "0 0 4 * * ?"
	}
	if strings.TrimSpace(cfg.UpdateTime) == "" {
		cfg.UpdateTime = "04:00"
	}
	return &Service{
		client: client,
		store:  store,
		cfg:    cfg,
		logger: logger,
		now:    time.Now,
	}
}

func (s *Service) IsKnownAirport(ctx context.Context, airportCode string) bool {
	airportCode = normalizeAirportCode(airportCode)
	if len(airportCode) != 3 {
		return false
	}
	ok, err := s.store.IsKnownAirport(ctx, airportCode)
	return err == nil && ok
}

func (s *Service) IsValidAirportRoute(ctx context.Context, originCode, destinationCode string) bool {
	originCode = normalizeAirportCode(originCode)
	destinationCode = normalizeAirportCode(destinationCode)
	if len(originCode) != 3 || len(destinationCode) != 3 || originCode == destinationCode {
		return false
	}
	ok, err := s.store.IsValidAirportRoute(ctx, originCode, destinationCode)
	return err == nil && ok
}

func (s *Service) IsValidCityRoute(ctx context.Context, originCode, destinationCode string) bool {
	originCode = normalizeAirportCode(originCode)
	destinationCode = normalizeAirportCode(destinationCode)
	if len(originCode) != 3 || len(destinationCode) != 3 || originCode == destinationCode {
		return false
	}
	ok, err := s.store.IsValidCityRoute(ctx, originCode, destinationCode)
	return err == nil && ok
}

func (s *Service) RefreshIfNeeded(ctx context.Context) error {
	needed, err := s.NeedsRefresh(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("error checking TF route cache update status", "error", err)
		}
		return err
	}
	if !needed {
		if s.logger != nil {
			s.logger.Info("TF route cache is up to date, no update needed")
		}
		return nil
	}
	return s.Refresh(ctx)
}

func (s *Service) Refresh(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("updating TF supplier route cache from TravelFusion API")
	}
	suppliers, err := s.client.GetBranchSupplierList(ctx)
	if err != nil {
		return fmt.Errorf("get branch supplier list: %w", err)
	}
	if len(suppliers) == 0 {
		if s.logger != nil {
			s.logger.Warn("no active plane suppliers returned from TravelFusion API, keeping existing cache")
		}
		return nil
	}

	airportRouteSet := map[string]struct{}{}
	cityRouteSet := map[string]struct{}{}
	failedSuppliers := make([]string, 0)
	for _, supplier := range suppliers {
		routes, err := s.client.ListSupplierRoutes(ctx, supplier, false)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to fetch TF supplier routes, skipping supplier", "supplier", supplier, "error", err)
			}
			failedSuppliers = append(failedSuppliers, supplier)
			continue
		}
		if routes == nil {
			continue
		}
		addRoutes(airportRouteSet, routes.AirportRoutes)
		addRoutes(cityRouteSet, routes.CityRoutes)
	}
	if len(airportRouteSet) == 0 && len(cityRouteSet) == 0 {
		if s.logger != nil {
			s.logger.Warn("no routes collected from any supplier, keeping existing cache")
		}
		return nil
	}

	airportRoutes := setToSortedSlice(airportRouteSet)
	cityRoutes := setToSortedSlice(cityRouteSet)
	knownAirports := knownAirportsFromRoutes(airportRoutes)
	if err := s.store.ReplaceRoutes(ctx, airportRoutes, cityRoutes, knownAirports); err != nil {
		return err
	}
	if err := s.store.SetLastUpdate(ctx, s.now()); err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Info(
			"TF route cache updated",
			"airport_routes", len(airportRoutes),
			"city_routes", len(cityRoutes),
			"suppliers_succeeded", len(suppliers)-len(failedSuppliers),
			"suppliers_total", len(suppliers),
			"failed_suppliers", failedSuppliers,
		)
	}
	return nil
}

func (s *Service) NeedsRefresh(ctx context.Context) (bool, error) {
	lastUpdate, err := s.store.LastUpdate(ctx)
	if err != nil {
		return false, err
	}
	if lastUpdate == nil {
		return true, nil
	}
	return lastUpdate.Before(s.latestScheduledUpdateTime()), nil
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.RefreshIfNeeded(ctx); err != nil && s.logger != nil {
		s.logger.Warn("failed to update TF supplier route cache, keeping existing cache", "error", err)
	}
	scheduler := cron.New(cron.WithSeconds())
	if _, err := scheduler.AddFunc(s.cfg.UpdateCron, func() {
		if s.logger != nil {
			s.logger.Info("scheduled TF route cache update triggered")
		}
		if err := s.RefreshIfNeeded(context.Background()); err != nil && s.logger != nil {
			s.logger.Warn("failed to update TF supplier route cache, keeping existing cache", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("schedule TF supplier route update: %w", err)
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

func (s *Service) latestScheduledUpdateTime() time.Time {
	parsed, err := time.Parse("15:04", strings.TrimSpace(s.cfg.UpdateTime))
	if err != nil {
		parsed = time.Date(0, 1, 1, 4, 0, 0, 0, time.Local)
	}
	now := s.now()
	todayAtUpdateTime := time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
	if now.Before(todayAtUpdateTime) {
		return todayAtUpdateTime.AddDate(0, 0, -1)
	}
	return todayAtUpdateTime
}
