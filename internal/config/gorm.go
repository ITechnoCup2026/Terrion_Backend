package config

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func buildDSN(cfg *Config) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.Port,
		cfg.Database.SSLMode,
	)
}

func NewDatabase(cfg *Config, log *logrus.Logger) *gorm.DB {
	if cfg.Database.Host == "" || cfg.Database.User == "" {
		log.Fatalf("DB_HOST and DB_USER are empty: no .env was found from %s up to the module root", workingDirectory())
	}

	dsn := buildDSN(cfg)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.New(&logrusWriter{Logger: log}, logger.Config{
			SlowThreshold:             time.Second * 5,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			LogLevel:                  logger.Info,
		}),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection, err := db.DB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection.SetMaxIdleConns(cfg.Database.PoolIdle)
	connection.SetMaxOpenConns(cfg.Database.PoolMax)
	connection.SetConnMaxLifetime(time.Second * time.Duration(cfg.Database.PoolLifetime))

	return db
}

type logrusWriter struct {
	Logger *logrus.Logger
}

func (l *logrusWriter) Printf(message string, args ...interface{}) {
	l.Logger.Tracef(message, args...)
}

func workingDirectory() string {
	directory, err := os.Getwd()
	if err != nil {
		return "the working directory"
	}
	return directory
}
