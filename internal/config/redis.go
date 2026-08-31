package config

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func NewRedis(cfg *Config, log *logrus.Logger) *redis.Client {
	if cfg.Redis.URL == "" {
		log.Fatalf("REDIS_URL is empty: no .env was found from %s up to the module root", workingDirectory())
	}

	options, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		log.Fatalf("failed to parse redis url: %v", err)
	}

	client := redis.NewClient(options)

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	return client
}
