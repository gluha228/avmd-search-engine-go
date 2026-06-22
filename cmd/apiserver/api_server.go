package main

import (
	"avmd-search-engine-go/internal/config"
	dbstore "avmd-search-engine-go/internal/db"
	"avmd-search-engine-go/internal/httpserver"
	"avmd-search-engine-go/internal/logging"
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
	if err := dbstore.RunMigrations(context.Background(), cfg, logger); err != nil {
		panic(err)
	}
	server := httpserver.NewHttpServer(cfg, logger)
	server.InitHandlers()

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("api server starting listening", "port", cfg.Port)
	if err := http.ListenAndServe(addr, server.CreateHandler()); err != nil {
		panic(err)
	}
}
