package config

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewLogger_SetsLevelFromConfig(t *testing.T) {
	cfg := &Config{}
	cfg.Log.Level = int(logrus.WarnLevel)

	log := NewLogger(cfg)

	if log.GetLevel() != logrus.WarnLevel {
		t.Errorf("log level = %v, want %v", log.GetLevel(), logrus.WarnLevel)
	}
}

func TestNewLogger_UsesJSONFormatter(t *testing.T) {
	cfg := &Config{}

	log := NewLogger(cfg)

	if _, ok := log.Formatter.(*logrus.JSONFormatter); !ok {
		t.Errorf("formatter = %T, want *logrus.JSONFormatter", log.Formatter)
	}
}
