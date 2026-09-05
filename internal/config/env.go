package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"

	"terrion-backend/internal/constants"
)

type Config struct {
	App struct {
		Name string
		Env  string
	}
	Web struct {
		Port        int
		Prefork     bool
		CorsOrigins string
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
	Cron struct {
		Secret string
	}
	Supabase struct {
		URL            string
		AnonKey        string
		ServiceRoleKey string
		JWTSecret      string
	}

	AI struct {
		ServiceURL     string
		Token          string
		Timeout        int
		WarmupInterval int
	}
}

func NewConfig() *Config {
	loadEnvFile()

	cfg := &Config{}

	cfg.App.Name = getEnv("APP_NAME", "terrion-backend")
	cfg.App.Env = getEnv("APP_ENV", "development")

	cfg.Web.Port = getEnvAsInt("PORT", getEnvAsInt("WEB_PORT", 8080))
	cfg.Web.Prefork = getEnvAsBool("WEB_PREFORK", false)
	cfg.Web.CorsOrigins = getEnv("WEB_CORS_ORIGINS", "http://localhost:3000")

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

	cfg.Cron.Secret = getEnv("CRON_SECRET", "")

	cfg.Supabase.URL = getEnv("SUPABASE_URL", "")
	cfg.Supabase.AnonKey = getEnv("SUPABASE_ANON_KEY", "")
	cfg.Supabase.ServiceRoleKey = getEnv("SUPABASE_SERVICE_ROLE_KEY", "")
	cfg.Supabase.JWTSecret = getEnv("SUPABASE_JWT_SECRET", "")

	cfg.AI.ServiceURL = getEnv("AI_SERVICE_URL", "")
	cfg.AI.Token = getEnv("AI_SERVICE_TOKEN", "")
	cfg.AI.Timeout = getEnvAsInt("AI_SERVICE_TIMEOUT_MS", 3500)
	cfg.AI.WarmupInterval = getEnvAsInt("AI_WARMUP_INTERVAL", 0)

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

func loadEnvFile() {
	directory, err := os.Getwd()
	if err != nil {
		return
	}

	for {
		if _, err := os.Stat(filepath.Join(directory, constants.EnvFileName)); err == nil {
			_ = godotenv.Load(filepath.Join(directory, constants.EnvFileName))
			return
		}
		if _, err := os.Stat(filepath.Join(directory, constants.ModuleFileName)); err == nil {
			return
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			return
		}
		directory = parent
	}
}
