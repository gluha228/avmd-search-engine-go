package main

import (
	"avmd-search-engine-go/internal/config"
	"avmd-search-engine-go/internal/httpserver"
	"avmd-search-engine-go/internal/logging"
	"fmt"
	"net/http"
)

func main() {
	cfg, err := config.GetFromEnv()
	if err != nil {
		panic(err)
	}

	logger := logging.NewLogger(cfg)
	server := httpserver.NewHttpServer(cfg, logger)
	server.InitHandlers()

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("api server starting listening", "port", cfg.Port)
	if err := http.ListenAndServe(addr, server.CreateHandler()); err != nil {
		panic(err)
	}
}
