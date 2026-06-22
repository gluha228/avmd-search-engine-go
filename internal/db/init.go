package db

import (
	"avmd-search-engine-go/internal/config"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateConnection(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return pool, err
	}
	return pool, nil
}
