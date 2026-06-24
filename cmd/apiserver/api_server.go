package main

import (
	"avmd-search-engine-go/internal/config"
	dbstore "avmd-search-engine-go/internal/db"
	"avmd-search-engine-go/internal/httpserver"
	"avmd-search-engine-go/internal/logging"
	"avmd-search-engine-go/internal/redishealth"
	"context"
	"fmt"
	"net/http"
)

func main() {
	cfg, err := config.GetFromEnv()
	if err != nil {
		panic(err)
	}

	logger := logging.NewLogger(cfg)
	redishealth.ConfigureLogger(logger)
	redisClient := redishealth.NewPreflightClient(cfg)
	if err := redishealth.Wait(context.Background(), redisClient, redishealth.WaitOptionsFromConfig(cfg), logger); err != nil {
		panic(err)
	}
	_ = redisClient.Close()
	if err := dbstore.RunMigrations(context.Background(), cfg, logger); err != nil {
		panic(err)
	}
	server := httpserver.NewHttpServer(cfg, logger)
	if err := server.InitHandlers(); err != nil {
		panic(err)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("api server starting listening", "port", cfg.Port)
	if err := http.ListenAndServe(addr, server.CreateHandler()); err != nil {
		panic(err)
	}
}
