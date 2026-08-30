package config

import "testing"

func TestNewConfig_Defaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.App.Name != "terrion-backend" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "terrion-backend")
	}
	if cfg.App.Env != "development" {
		t.Errorf("App.Env = %q, want %q", cfg.App.Env, "development")
	}
	if cfg.Web.Port != 8080 {
		t.Errorf("Web.Port = %d, want %d", cfg.Web.Port, 8080)
	}
	if cfg.Web.Prefork != false {
		t.Errorf("Web.Prefork = %v, want %v", cfg.Web.Prefork, false)
	}
	if cfg.Log.Level != 4 {
		t.Errorf("Log.Level = %d, want %d", cfg.Log.Level, 4)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want %d", cfg.Database.Port, 5432)
	}
	if cfg.Database.Name != "postgres" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "postgres")
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("Database.SSLMode = %q, want %q", cfg.Database.SSLMode, "require")
	}
	if cfg.Database.PoolIdle != 10 {
		t.Errorf("Database.PoolIdle = %d, want %d", cfg.Database.PoolIdle, 10)
	}
	if cfg.Database.PoolMax != 100 {
		t.Errorf("Database.PoolMax = %d, want %d", cfg.Database.PoolMax, 100)
	}
	if cfg.Database.PoolLifetime != 300 {
		t.Errorf("Database.PoolLifetime = %d, want %d", cfg.Database.PoolLifetime, 300)
	}
}

func TestNewConfig_Overrides(t *testing.T) {
	t.Setenv("APP_NAME", "custom-app")
	t.Setenv("WEB_PORT", "9090")
	t.Setenv("WEB_PREFORK", "true")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_POOL_MAX", "50")

	cfg := NewConfig()

	if cfg.App.Name != "custom-app" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "custom-app")
	}
	if cfg.Web.Port != 9090 {
		t.Errorf("Web.Port = %d, want %d", cfg.Web.Port, 9090)
	}
	if cfg.Web.Prefork != true {
		t.Errorf("Web.Prefork = %v, want %v", cfg.Web.Prefork, true)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "db.example.com")
	}
	if cfg.Database.PoolMax != 50 {
		t.Errorf("Database.PoolMax = %d, want %d", cfg.Database.PoolMax, 50)
	}
}
