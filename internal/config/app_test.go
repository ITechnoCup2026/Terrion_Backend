package config

import "testing"

func TestBootstrap_WiresRouteConfigWithoutPanicking(t *testing.T) {
	cfg := &Config{}
	cfg.App.Name = "test-app"

	app := NewFiber(cfg)
	log := NewLogger(cfg)
	validate := NewValidator()

	Bootstrap(&BootstrapConfig{
		DB:       nil,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
	})
}
