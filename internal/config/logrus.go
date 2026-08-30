package config

import "github.com/sirupsen/logrus"

func NewLogger(cfg *Config) *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.Level(cfg.Log.Level))
	log.SetFormatter(&logrus.JSONFormatter{})
	return log
}
