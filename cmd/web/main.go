package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/config"
)

func main() {
	cfg := config.NewConfig()
	log := config.NewLogger(cfg)
	db := config.NewDatabase(cfg, log)
	rdb := config.NewRedis(cfg, log)
	validate := config.NewValidator()
	app := config.NewFiber(cfg)

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		Redis:    rdb,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
	})

	if cfg.AI.ServiceURL != "" && cfg.AI.WarmupInterval > 0 {
		go warmAIService(cfg, log)
	}

	err := app.Listen(fmt.Sprintf(":%d", cfg.Web.Port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func warmAIService(cfg *config.Config, log *logrus.Logger) {
	client := aiclient.NewClient(
		cfg.AI.ServiceURL, cfg.AI.Token,
		time.Duration(cfg.AI.Timeout)*time.Millisecond)
	if client == nil {
		return
	}

	ticker := time.NewTicker(time.Duration(cfg.AI.WarmupInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := client.Health(context.Background()); err != nil {
			log.WithError(err).Debug("pemanasan layanan AI gagal")
		}
	}
}
