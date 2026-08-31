package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"terrion-backend/internal/config"
	"terrion-backend/internal/constants"
)

func main() {
	source := flag.String("path", constants.MigrationsPath, "directory holding the migrations")
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		usage()
		os.Exit(2)
	}

	cfg := config.NewConfig()
	log := config.NewLogger(cfg)

	runner, err := migrate.New("file://"+*source, postgresURL(cfg))
	if err != nil {
		log.Fatalf("opening migrations at %s: %v", *source, err)
	}
	defer runner.Close()

	if err := run(runner, command, flag.Arg(1)); err != nil {
		log.Fatalf("%s: %v", command, err)
	}

	version, dirty, err := runner.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		log.Info("database has no migrations applied")
		return
	}
	if err != nil {
		log.Fatalf("reading the applied version: %v", err)
	}
	log.Infof("database is at version %d (dirty: %t)", version, dirty)
}

func run(runner *migrate.Migrate, command, argument string) error {
	switch command {
	case "up":
		return skipNoChange(runner.Up())
	case "down":
		return skipNoChange(runner.Steps(-1))
	case "drop":
		return runner.Drop()
	case "force":
		version, err := strconv.Atoi(argument)
		if err != nil {
			return fmt.Errorf("force needs a version number, got %q", argument)
		}
		return runner.Force(version)
	case "version":
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func skipNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func postgresURL(cfg *config.Config) string {
	query := url.Values{}
	query.Set("sslmode", cfg.Database.SSLMode)

	address := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.Database.User, cfg.Database.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Database.Host, cfg.Database.Port),
		Path:     cfg.Database.Name,
		RawQuery: query.Encode(),
	}
	return address.String()
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: go run cmd/migrate/main.go [-path dir] <command>

  up                apply every pending migration
  down              roll back the most recently applied migration
  force <version>   mark the database as being at <version> without running anything
  drop              delete every table in the database
  version           report the applied version

Point DB_* in .env at the target database. Use force on a database that already
has the schema from another source, so up does not try to create it again.
`)
}
