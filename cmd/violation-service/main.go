package main

import (
	"fmt"
	"os"

	"violation-service/internal/auth"
	"violation-service/internal/config"
	"violation-service/internal/db"
	httphandler "violation-service/internal/http"
	"violation-service/internal/http/middleware"
	"violation-service/internal/logger"
	"violation-service/internal/repository"
	"violation-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Environment)

	database, err := db.New(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	scopeRepo := repository.NewScopeRepository(database)
	violationRepo := repository.NewViolationRepository(database)
	appealRepo := repository.NewAppealRepository(database)

	violationService := service.NewViolationService(scopeRepo, violationRepo, appealRepo)
	appealService := service.NewAppealService(scopeRepo, violationRepo, appealRepo, cfg.Files.MaxAttachmentsPerAction)

	tokenParser := auth.NewParser(cfg.Auth.AccessSecret)

	handler := httphandler.NewHandler(violationService, appealService, log)
	router := httphandler.NewRouter(handler, middleware.Auth(tokenParser), cfg.Environment)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	log.Info().Str("addr", addr).Msg("starting violations service")

	if err := router.Run(addr); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
