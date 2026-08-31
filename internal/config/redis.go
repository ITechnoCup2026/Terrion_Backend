package config

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func NewRedis(cfg *Config, log *logrus.Logger) *redis.Client {
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
