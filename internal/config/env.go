package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	App struct {
		Name string
		Env  string
	}
	Web struct {
		Port    int
		Prefork bool
	}
	Log struct {
		Level int
	}
	Database struct {
		Host         string
		Port         int
		User         string
		Password     string
		Name         string
		SSLMode      string
		PoolIdle     int
		PoolMax      int
		PoolLifetime int
	}
	Redis struct {
		URL string
	}
}

// NewConfig loads .env into the process environment (if present) and builds
// a typed Config from environment variables, falling back to defaults.
func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.App.Name = getEnv("APP_NAME", "terrion-backend")
	cfg.App.Env = getEnv("APP_ENV", "development")

	cfg.Web.Port = getEnvAsInt("WEB_PORT", 8080)
	cfg.Web.Prefork = getEnvAsBool("WEB_PREFORK", false)

	cfg.Log.Level = getEnvAsInt("LOG_LEVEL", 4)

	cfg.Database.Host = getEnv("DB_HOST", "")
	cfg.Database.Port = getEnvAsInt("DB_PORT", 5432)
	cfg.Database.User = getEnv("DB_USER", "")
	cfg.Database.Password = getEnv("DB_PASSWORD", "")
	cfg.Database.Name = getEnv("DB_NAME", "postgres")
	cfg.Database.SSLMode = getEnv("DB_SSLMODE", "require")
	cfg.Database.PoolIdle = getEnvAsInt("DB_POOL_IDLE", 10)
	cfg.Database.PoolMax = getEnvAsInt("DB_POOL_MAX", 100)
	cfg.Database.PoolLifetime = getEnvAsInt("DB_POOL_LIFETIME", 300)

	cfg.Redis.URL = getEnv("REDIS_URL", "")

	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}
