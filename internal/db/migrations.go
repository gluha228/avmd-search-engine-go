package db

import (
	"avmd-search-engine-go/internal/config"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const migrationsPath = "db/migrations"

func RunMigrations(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if !cfg.DatabaseAutoMigrate {
		return nil
	}
	if err := runSchemaMigrations(cfg.DatabaseURL, logger); err != nil {
		return err
	}
	pool, err := CreateConnection(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	return seedGeoDataIfNeeded(ctx, pool, logger)
}

func runSchemaMigrations(databaseURL string, logger *slog.Logger) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration db connection: %w", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migration driver: %w", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer migrator.Close()

	if err := migrator.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			if logger != nil {
				logger.Debug("database migrations are up to date")
			}
			return nil
		}
		return fmt.Errorf("run database migrations: %w", err)
	}
	if logger != nil {
		logger.Info("database migrations applied")
	}
	return nil
}

func seedGeoDataIfNeeded(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	countriesCount, err := tableCount(ctx, pool, "countries")
	if err != nil {
		return err
	}
	if countriesCount > 0 {
		return nil
	}
	if err := copyCSV(ctx, pool, "countries", []string{"id", "name_en", "name_ro", "name_ru", "iso3", "iso2"}, "db/seeders/countries/countries.csv", countryValues); err != nil {
		return err
	}
	if err := copyCSV(ctx, pool, "cities", []string{"id", "country_id", "name_en", "name_ro", "name_ru", "is_capital", "population", "timezone"}, "db/seeders/cities/cities.csv", cityValues); err != nil {
		return err
	}
	if err := copyCSV(ctx, pool, "airports", []string{"id", "city_id", "iata_code", "icao_code", "lat", "lon"}, "db/seeders/airports/airports.csv", airportValues); err != nil {
		return err
	}
	for _, table := range []string{"countries", "cities", "airports"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s', 'id'), COALESCE(MAX(id), 1), true) FROM %s", table, table)); err != nil {
			return fmt.Errorf("reset %s sequence: %w", table, err)
		}
	}
	if logger != nil {
		logger.Info("geo seed data loaded")
	}
	return nil
}

func tableCount(ctx context.Context, pool *pgxpool.Pool, table string) (int64, error) {
	var count int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return count, nil
}

func copyCSV(
	ctx context.Context,
	pool *pgxpool.Pool,
	table string,
	columns []string,
	path string,
	mapRow func([]string) ([]any, error),
) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("read %s header: %w", path, err)
	}
	rows := make([][]any, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		values, err := mapRow(record)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		rows = append(rows, values)
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}

func countryValues(row []string) ([]any, error) {
	id, err := parseInt64(row[0])
	if err != nil {
		return nil, err
	}
	return []any{id, row[1], row[2], row[3], row[4], row[5]}, nil
}

func cityValues(row []string) ([]any, error) {
	id, err := parseInt64(row[0])
	if err != nil {
		return nil, err
	}
	countryID, err := parseInt64(row[1])
	if err != nil {
		return nil, err
	}
	isCapital, err := strconv.ParseBool(strings.TrimSpace(row[5]))
	if err != nil {
		return nil, err
	}
	population, err := parseNullableInt64(row[6])
	if err != nil {
		return nil, err
	}
	timezone := nullableString(row[7])
	return []any{id, countryID, row[2], row[3], row[4], isCapital, population, timezone}, nil
}

func airportValues(row []string) ([]any, error) {
	id, err := parseInt64(row[0])
	if err != nil {
		return nil, err
	}
	cityID, err := parseInt64(row[1])
	if err != nil {
		return nil, err
	}
	lat, err := parseNullableFloat64(row[4])
	if err != nil {
		return nil, err
	}
	lon, err := parseNullableFloat64(row[5])
	if err != nil {
		return nil, err
	}
	return []any{id, cityID, nullableString(row[2]), nullableString(row[3]), lat, lon}, nil
}

func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func parseNullableInt64(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseNullableFloat64(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
