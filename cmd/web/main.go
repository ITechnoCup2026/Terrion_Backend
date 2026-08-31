package main

import (
	"fmt"

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

	err := app.Listen(fmt.Sprintf(":%d", cfg.Web.Port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
