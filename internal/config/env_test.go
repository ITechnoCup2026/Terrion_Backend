package config

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	isolatedModule = "module isolated\n"
	rootEnvFile    = "APP_NAME=from-root\n"
)

func isolateEnvironment(t *testing.T) {
	t.Helper()

	directory := t.TempDir()
	module := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(module, []byte(isolatedModule), 0o600); err != nil {
		t.Fatalf("writing %s: %v", module, err)
	}
	t.Chdir(directory)

	for _, name := range []string{
		"APP_NAME", "APP_ENV", "PORT", "WEB_PORT", "WEB_PREFORK", "WEB_CORS_ORIGINS",
		"LOG_LEVEL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"DB_SSLMODE", "DB_POOL_IDLE", "DB_POOL_MAX", "DB_POOL_LIFETIME",
		"REDIS_URL", "CRON_SECRET",
		"SUPABASE_URL", "SUPABASE_ANON_KEY", "SUPABASE_SERVICE_ROLE_KEY",
		"SUPABASE_JWT_SECRET",
		"AI_SERVICE_URL", "AI_SERVICE_TOKEN", "AI_SERVICE_TIMEOUT_MS",
		"AI_WARMUP_INTERVAL",
	} {
		t.Setenv(name, "")
	}
}

func TestNewConfig_Defaults(t *testing.T) {
	isolateEnvironment(t)

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
	isolateEnvironment(t)

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

func TestNewConfigPrefersRailwaysPortOverWebPort(t *testing.T) {
	isolateEnvironment(t)
	t.Setenv("WEB_PORT", "8080")
	t.Setenv("PORT", "3333")

	if got := NewConfig().Web.Port; got != 3333 {
		t.Errorf("Web.Port = %d, want 3333: the platform's PORT wins", got)
	}
}

func TestNewConfigFindsTheEnvFileFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"), []byte(isolatedModule), 0o600); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, ".env"), []byte(rootEnvFile), 0o600); err != nil {
		t.Fatalf("writing .env: %v", err)
	}

	nested := filepath.Join(root, "cmd", "web")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("creating %s: %v", nested, err)
	}

	t.Chdir(nested)

	if got := NewConfig().App.Name; got != "from-root" {
		t.Errorf("App.Name = %q, want from-root: .env lives at the module root", got)
	}
}
