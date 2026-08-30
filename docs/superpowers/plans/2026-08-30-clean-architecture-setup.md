# Terrion Backend Clean Architecture Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the Terrion Backend Go project with a layered clean-architecture folder structure (adapted from khannedy/golang-clean-architecture), `.env`-based configuration, Supabase Postgres wiring, an empty `internal/constants/` folder, and a `PROJECT_STRUCTURE.md` doc — no Kafka, no worker process, no business domain yet.

**Architecture:** `cmd/web/main.go` builds config/logger/db/validator/fiber-app, then hands them to `config.Bootstrap()`, the composition root, which (once domains exist) wires repository → usecase → controller → route. Repositories take `*gorm.DB` per call so usecases own the transaction boundary (not implemented here since no domain exists, but the generic `Repository[T]` base and the wiring point are in place). Config is env-var driven via a typed `*config.Config` struct loaded through `godotenv` + `os.Getenv`, replacing the reference's Viper/JSON.

**Tech Stack:** Go 1.25, Fiber v2, GORM + `gorm.io/driver/postgres` (Supabase), go-playground/validator v10, logrus, godotenv, glebarez/sqlite (test-only, pure-Go, for the generic repository test).

**Spec:** `docs/superpowers/specs/2026-08-30-clean-architecture-setup-design.md`

## Global Constraints

- Go module path is exactly `terrion-backend`; `go.mod` declares `go 1.25` (matches the installed toolchain, `go1.25.6`).
- No Kafka, no `cmd/worker`, no `delivery/messaging`, no `gateway/messaging` — do not introduce them.
- Configuration is `.env`-only: no Viper, no `config.json`. Env var loading uses `github.com/joho/godotenv` + `os.Getenv`.
- Database is Supabase Postgres via `gorm.io/driver/postgres`, targeting the session pooler / direct connection (port `5432`), `sslmode=require` by default. Prepared statements stay enabled (no `PreferSimpleProtocol`).
- `internal/constants/` is created empty — no seed content.
- No business domain/entity code in this plan (bare skeleton) — `internal/entity/`, `internal/model/converter/`, `internal/usecase/`, `internal/delivery/http/middleware/` stay empty (tracked via `.gitkeep`).
- Windows development machine: avoid CGO-dependent test dependencies. Use `github.com/glebarez/sqlite` (pure Go) for the repository test's in-memory DB, not `mattn/go-sqlite3`.
- Every task ends with a commit. Follow existing file naming convention for any future domain code: `xxx_entity.go`, `xxx_model.go`, `xxx_converter.go`, `xxx_repository.go`, `xxx_usecase.go`, `xxx_controller.go` (documented in `PROJECT_STRUCTURE.md`, not exercised here).

---

## File Structure

```
Terrion_Backend/
├── cmd/web/main.go                          # entrypoint (Task 11)
├── internal/
│   ├── config/
│   │   ├── env.go / env_test.go             # Config struct + loader (Task 2)
│   │   ├── logrus.go / logrus_test.go       # NewLogger (Task 3)
│   │   ├── validator.go / validator_test.go # NewValidator (Task 4)
│   │   ├── fiber.go / fiber_test.go         # NewFiber + error handler (Task 5)
│   │   ├── gorm.go / gorm_test.go           # NewDatabase + buildDSN (Task 6)
│   │   └── app.go / app_test.go             # BootstrapConfig + Bootstrap (Task 10)
│   ├── constants/.gitkeep                   # empty (Task 1)
│   ├── delivery/http/
│   │   ├── middleware/.gitkeep              # empty (Task 1)
│   │   └── route/route.go / route_test.go   # RouteConfig + Setup (Task 9)
│   ├── entity/.gitkeep                      # empty (Task 1)
│   ├── model/
│   │   ├── converter/.gitkeep               # empty (Task 1)
│   │   └── model.go / model_test.go         # WebResponse/PageResponse (Task 7)
│   ├── repository/repository.go / repository_test.go  # generic Repository[T] (Task 8)
│   └── usecase/.gitkeep                     # empty (Task 1)
├── db/migrations/.gitkeep                   # empty (Task 1)
├── .env.example                             # config keys documented (Task 2)
├── .gitignore                               # includes .env (Task 1)
├── go.mod / go.sum                          # (Task 1, updated throughout)
└── PROJECT_STRUCTURE.md                     # architecture doc for Claude Code (Task 12)
```

---

### Task 1: Module init, folder skeleton, .gitignore

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `internal/constants/.gitkeep`
- Create: `internal/entity/.gitkeep`
- Create: `internal/model/converter/.gitkeep`
- Create: `internal/usecase/.gitkeep`
- Create: `internal/delivery/http/middleware/.gitkeep`
- Create: `db/migrations/.gitkeep`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: Go module named `terrion-backend` that later tasks add packages under (`terrion-backend/internal/config`, etc.); the empty directories other tasks will eventually populate.

- [ ] **Step 1: Initialize the Go module**

Run: `go mod init terrion-backend`
Expected: creates `go.mod` with `module terrion-backend` and `go 1.25` (matches installed `go1.25.6`).

- [ ] **Step 2: Create `.gitignore`**

```gitignore
# Environment
.env

# Build artifacts
/bin/
/tmp/

# IDE
.idea/
.vscode/

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 3: Create placeholder files for empty directories**

Git does not track empty directories, so add a `.gitkeep` to each folder that has no code yet:

```bash
mkdir -p internal/constants internal/entity internal/model/converter internal/usecase internal/delivery/http/middleware db/migrations
touch internal/constants/.gitkeep internal/entity/.gitkeep internal/model/converter/.gitkeep internal/usecase/.gitkeep internal/delivery/http/middleware/.gitkeep db/migrations/.gitkeep
```

- [ ] **Step 4: Verify**

Run: `cat go.mod`
Expected: contains `module terrion-backend` and `go 1.25`.

Run: `git status --short`
Expected: shows `go.mod`, `.gitignore`, and the six new `.gitkeep` files as untracked.

- [ ] **Step 5: Commit**

```bash
git add go.mod .gitignore internal/constants/.gitkeep internal/entity/.gitkeep internal/model/converter/.gitkeep internal/usecase/.gitkeep internal/delivery/http/middleware/.gitkeep db/migrations/.gitkeep
git commit -m "chore: initialize Go module and clean architecture folder skeleton"
```

---

### Task 2: Env-based configuration (`internal/config`)

**Files:**
- Create: `internal/config/env.go`
- Create: `internal/config/env_test.go`
- Create: `.env.example`

**Interfaces:**
- Consumes: nothing beyond the standard library and `github.com/joho/godotenv`.
- Produces: `type Config struct { App struct{Name, Env string}; Web struct{Port int; Prefork bool}; Log struct{Level int}; Database struct{Host, User, Password, Name, SSLMode string; Port, PoolIdle, PoolMax, PoolLifetime int} }` and `func NewConfig() *Config`. Every later task in `internal/config` takes `*Config` as a parameter and reads these exact field paths (`cfg.App.Name`, `cfg.Web.Port`, `cfg.Web.Prefork`, `cfg.Log.Level`, `cfg.Database.Host/Port/User/Password/Name/SSLMode/PoolIdle/PoolMax/PoolLifetime`).

- [ ] **Step 1: Add the godotenv dependency**

Run: `go get github.com/joho/godotenv`

- [ ] **Step 2: Write the failing test**

Create `internal/config/env_test.go`:

```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestNewConfig -v`
Expected: FAIL — `NewConfig` (and `Config`) undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/env.go`:

```go
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestNewConfig -v`
Expected: PASS for both `TestNewConfig_Defaults` and `TestNewConfig_Overrides`.

- [ ] **Step 6: Create `.env.example`**

Create `.env.example` at the repo root:

```dotenv
# Application
APP_NAME=terrion-backend
APP_ENV=development

# Web server (Fiber)
WEB_PORT=8080
WEB_PREFORK=false

# Logging (logrus levels: 0=Panic 1=Fatal 2=Error 3=Warn 4=Info 5=Debug 6=Trace)
LOG_LEVEL=4

# Database (Supabase Postgres)
# Get these from your Supabase project: Project Settings -> Database.
# This targets the session pooler / direct connection (port 5432), NOT the
# transaction pooler (port 6543) — prepared statements stay enabled.
DB_HOST=
DB_PORT=5432
DB_USER=
DB_PASSWORD=
DB_NAME=postgres
DB_SSLMODE=require
DB_POOL_IDLE=10
DB_POOL_MAX=100
DB_POOL_LIFETIME=300
```

- [ ] **Step 7: Commit**

```bash
git add internal/config/env.go internal/config/env_test.go .env.example go.mod go.sum
git commit -m "feat: add .env-based Config loader"
```

---

### Task 3: Logger (`internal/config/logrus.go`)

**Files:**
- Create: `internal/config/logrus.go`
- Create: `internal/config/logrus_test.go`

**Interfaces:**
- Consumes: `*Config` from Task 2 (`cfg.Log.Level`).
- Produces: `func NewLogger(cfg *Config) *logrus.Logger`. Consumed by Task 6 (`NewDatabase`), Task 10 (`Bootstrap`/`BootstrapConfig.Log`), and Task 11 (`main.go`).

- [ ] **Step 1: Add the logrus dependency**

Run: `go get github.com/sirupsen/logrus`

- [ ] **Step 2: Write the failing test**

Create `internal/config/logrus_test.go`:

```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestNewLogger -v`
Expected: FAIL — `NewLogger` undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/logrus.go`:

```go
package config

import "github.com/sirupsen/logrus"

func NewLogger(cfg *Config) *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.Level(cfg.Log.Level))
	log.SetFormatter(&logrus.JSONFormatter{})
	return log
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestNewLogger -v`
Expected: PASS for both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/config/logrus.go internal/config/logrus_test.go go.mod go.sum
git commit -m "feat: add logrus logger config"
```

---

### Task 4: Validator (`internal/config/validator.go`)

**Files:**
- Create: `internal/config/validator.go`
- Create: `internal/config/validator_test.go`

**Interfaces:**
- Consumes: nothing beyond `github.com/go-playground/validator/v10`.
- Produces: `func NewValidator() *validator.Validate`. Consumed by Task 10 (`Bootstrap`/`BootstrapConfig.Validate`) and Task 11 (`main.go`).

- [ ] **Step 1: Add the validator dependency**

Run: `go get github.com/go-playground/validator/v10`

- [ ] **Step 2: Write the failing test**

Create `internal/config/validator_test.go`:

```go
package config

import "testing"

type sampleValidatorRequest struct {
	Name string `validate:"required"`
}

func TestNewValidator_ValidatesRequiredField(t *testing.T) {
	validate := NewValidator()

	if err := validate.Struct(&sampleValidatorRequest{Name: ""}); err == nil {
		t.Fatal("expected validation error for empty required field, got nil")
	}

	if err := validate.Struct(&sampleValidatorRequest{Name: "ok"}); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestNewValidator -v`
Expected: FAIL — `NewValidator` undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/validator.go`:

```go
package config

import "github.com/go-playground/validator/v10"

func NewValidator() *validator.Validate {
	return validator.New()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestNewValidator -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/validator.go internal/config/validator_test.go go.mod go.sum
git commit -m "feat: add go-playground validator config"
```

---

### Task 5: Fiber app + error handler (`internal/config/fiber.go`)

**Files:**
- Create: `internal/config/fiber.go`
- Create: `internal/config/fiber_test.go`

**Interfaces:**
- Consumes: `*Config` from Task 2 (`cfg.App.Name`, `cfg.Web.Prefork`).
- Produces: `func NewFiber(cfg *Config) *fiber.App` and `func NewErrorHandler() fiber.ErrorHandler`. Consumed by Task 9 (`route.RouteConfig.App`), Task 10 (`Bootstrap`/`BootstrapConfig.App`), Task 11 (`main.go`).

- [ ] **Step 1: Add the Fiber dependency**

Run: `go get github.com/gofiber/fiber/v2`

- [ ] **Step 2: Write the failing test**

Create `internal/config/fiber_test.go`:

```go
package config

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestNewFiber_ErrorHandlerFormatsFiberErrors(t *testing.T) {
	cfg := &Config{}
	cfg.App.Name = "test-app"
	cfg.Web.Prefork = false

	app := NewFiber(cfg)
	app.Get("/boom", func(ctx *fiber.Ctx) error {
		return fiber.ErrBadRequest
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "errors") {
		t.Errorf("body = %q, want it to contain %q", string(body), "errors")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestNewFiber -v`
Expected: FAIL — `NewFiber` undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/fiber.go`:

```go
package config

import "github.com/gofiber/fiber/v2"

func NewFiber(cfg *Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ErrorHandler: NewErrorHandler(),
		Prefork:      cfg.Web.Prefork,
	})

	return app
}

func NewErrorHandler() fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}

		return ctx.Status(code).JSON(fiber.Map{
			"errors": err.Error(),
		})
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestNewFiber -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/config/fiber.go internal/config/fiber_test.go go.mod go.sum
git commit -m "feat: add Fiber app config with JSON error handler"
```

---

### Task 6: Database connection (`internal/config/gorm.go`)

**Files:**
- Create: `internal/config/gorm.go`
- Create: `internal/config/gorm_test.go`

**Interfaces:**
- Consumes: `*Config` from Task 2 (all `cfg.Database.*` fields), `*logrus.Logger` from Task 3.
- Produces: `func NewDatabase(cfg *Config, log *logrus.Logger) *gorm.DB` and the pure helper `func buildDSN(cfg *Config) string`. Consumed by Task 10 (`Bootstrap`/`BootstrapConfig.DB`) and Task 11 (`main.go`).

- [ ] **Step 1: Add the GORM + Postgres driver dependencies**

Run: `go get gorm.io/gorm gorm.io/driver/postgres`

- [ ] **Step 2: Write the failing test**

Create `internal/config/gorm_test.go`:

```go
package config

import "testing"

func TestBuildDSN(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Host = "db.supabase.co"
	cfg.Database.User = "postgres"
	cfg.Database.Password = "secret"
	cfg.Database.Name = "postgres"
	cfg.Database.Port = 5432
	cfg.Database.SSLMode = "require"

	got := buildDSN(cfg)
	want := "host=db.supabase.co user=postgres password=secret dbname=postgres port=5432 sslmode=require"

	if got != want {
		t.Errorf("buildDSN() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestBuildDSN -v`
Expected: FAIL — `buildDSN` undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/config/gorm.go`:

```go
package config

import (
	"fmt"
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

// NewDatabase opens a connection to Supabase Postgres (session pooler /
// direct connection) and configures the connection pool.
func NewDatabase(cfg *Config, log *logrus.Logger) *gorm.DB {
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestBuildDSN -v`
Expected: PASS. (`NewDatabase` itself is not unit tested here — it requires a live Postgres connection; `buildDSN` isolates the testable logic. `NewDatabase` is exercised manually in Task 11's verification once real Supabase credentials are supplied.)

- [ ] **Step 6: Commit**

```bash
git add internal/config/gorm.go internal/config/gorm_test.go go.mod go.sum
git commit -m "feat: add Supabase Postgres GORM connection config"
```

---

### Task 7: Framework-level response models (`internal/model/model.go`)

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/model/model_test.go`

**Interfaces:**
- Consumes: nothing (standard library only).
- Produces: `type WebResponse[T any] struct { Data T; Paging *PageMetadata; Errors string }`, `type PageResponse[T any] struct { Data []T; PageMetadata PageMetadata }`, `type PageMetadata struct { Page, Size int; TotalItem, TotalPage int64 }`. These are the shared envelope types every future domain's controller will return — not consumed elsewhere in this plan since no controller exists yet.

- [ ] **Step 1: Write the failing test**

Create `internal/model/model_test.go`:

```go
package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebResponse_OmitsEmptyErrors(t *testing.T) {
	resp := WebResponse[string]{Data: "ok"}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	if strings.Contains(string(b), "errors") {
		t.Errorf("json = %s, expected no \"errors\" key when Errors is empty", b)
	}
	if !strings.Contains(string(b), `"data":"ok"`) {
		t.Errorf("json = %s, expected data field", b)
	}
}

func TestWebResponse_IncludesErrorsWhenSet(t *testing.T) {
	resp := WebResponse[string]{Errors: "bad request"}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	if !strings.Contains(string(b), `"errors":"bad request"`) {
		t.Errorf("json = %s, expected errors field", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/... -v`
Expected: FAIL — `WebResponse` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/model/model.go`:

```go
package model

type WebResponse[T any] struct {
	Data   T             `json:"data"`
	Paging *PageMetadata `json:"paging,omitempty"`
	Errors string        `json:"errors,omitempty"`
}

type PageResponse[T any] struct {
	Data         []T          `json:"data,omitempty"`
	PageMetadata PageMetadata `json:"paging,omitempty"`
}

type PageMetadata struct {
	Page      int   `json:"page"`
	Size      int   `json:"size"`
	TotalItem int64 `json:"total_item"`
	TotalPage int64 `json:"total_page"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/model/model.go internal/model/model_test.go
git commit -m "feat: add shared WebResponse/PageResponse models"
```

---

### Task 8: Generic repository base (`internal/repository/repository.go`)

**Files:**
- Create: `internal/repository/repository.go`
- Create: `internal/repository/repository_test.go`

**Interfaces:**
- Consumes: `gorm.io/gorm` (from Task 6's dependency addition; no dependency on any other task's code).
- Produces: `type Repository[T any] struct { DB *gorm.DB }` with methods `Create(db *gorm.DB, entity *T) error`, `Update(db *gorm.DB, entity *T) error`, `Delete(db *gorm.DB, entity *T) error`, `CountById(db *gorm.DB, id any) (int64, error)`, `FindById(db *gorm.DB, entity *T, id any) error`. Future domain repositories embed this (e.g. `type ProductRepository struct { repository.Repository[entity.Product] }`) — not exercised in this plan since no domain exists.

- [ ] **Step 1: Add the pure-Go sqlite driver (test-only)**

Run: `go get github.com/glebarez/sqlite`

- [ ] **Step 2: Write the failing test**

Create `internal/repository/repository_test.go`:

```go
package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testItem struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	if err := db.AutoMigrate(&testItem{}); err != nil {
		t.Fatalf("failed to migrate testItem: %v", err)
	}

	return db
}

func TestRepository_CreateAndFindById(t *testing.T) {
	db := setupTestDB(t)
	repo := &Repository[testItem]{}

	item := &testItem{Name: "widget"}
	if err := repo.Create(db, item); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("expected ID to be set after Create")
	}

	found := new(testItem)
	if err := repo.FindById(db, found, item.ID); err != nil {
		t.Fatalf("FindById error: %v", err)
	}
	if found.Name != "widget" {
		t.Errorf("found.Name = %q, want %q", found.Name, "widget")
	}
}

func TestRepository_UpdateDeleteAndCount(t *testing.T) {
	db := setupTestDB(t)
	repo := &Repository[testItem]{}

	item := &testItem{Name: "widget"}
	if err := repo.Create(db, item); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	item.Name = "updated-widget"
	if err := repo.Update(db, item); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	total, err := repo.CountById(db, item.ID)
	if err != nil {
		t.Fatalf("CountById error: %v", err)
	}
	if total != 1 {
		t.Errorf("CountById = %d, want 1", total)
	}

	if err := repo.Delete(db, item); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	total, err = repo.CountById(db, item.ID)
	if err != nil {
		t.Fatalf("CountById after delete error: %v", err)
	}
	if total != 0 {
		t.Errorf("CountById after delete = %d, want 0", total)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/repository/... -v`
Expected: FAIL — `Repository` undefined.

- [ ] **Step 4: Write the implementation**

Create `internal/repository/repository.go`:

```go
package repository

import "gorm.io/gorm"

type Repository[T any] struct {
	DB *gorm.DB
}

func (r *Repository[T]) Create(db *gorm.DB, entity *T) error {
	return db.Create(entity).Error
}

func (r *Repository[T]) Update(db *gorm.DB, entity *T) error {
	return db.Save(entity).Error
}

func (r *Repository[T]) Delete(db *gorm.DB, entity *T) error {
	return db.Delete(entity).Error
}

func (r *Repository[T]) CountById(db *gorm.DB, id any) (int64, error) {
	var total int64
	err := db.Model(new(T)).Where("id = ?", id).Count(&total).Error
	return total, err
}

func (r *Repository[T]) FindById(db *gorm.DB, entity *T, id any) error {
	return db.Where("id = ?", id).Take(entity).Error
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/repository/... -v`
Expected: PASS for both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/repository.go internal/repository/repository_test.go go.mod go.sum
git commit -m "feat: add generic Repository[T] base"
```

---

### Task 9: HTTP route skeleton (`internal/delivery/http/route/route.go`)

**Files:**
- Create: `internal/delivery/http/route/route.go`
- Create: `internal/delivery/http/route/route_test.go`

**Interfaces:**
- Consumes: `*fiber.App` (from Task 5's dependency, `github.com/gofiber/fiber/v2`).
- Produces: `type RouteConfig struct { App *fiber.App }` and `func (c *RouteConfig) Setup()`. Consumed by Task 10 (`Bootstrap`).

- [ ] **Step 1: Write the failing test**

Create `internal/delivery/http/route/route_test.go`:

```go
package route

import (
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRouteConfig_SetupDoesNotPanic(t *testing.T) {
	cfg := &RouteConfig{App: fiber.New()}

	cfg.Setup()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/delivery/http/route/... -v`
Expected: FAIL — `RouteConfig` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/delivery/http/route/route.go`:

```go
package route

import "github.com/gofiber/fiber/v2"

type RouteConfig struct {
	App *fiber.App
}

// Setup registers all HTTP routes. It is intentionally empty until the
// first domain controller exists — add routes here following the pattern
// documented in PROJECT_STRUCTURE.md.
func (c *RouteConfig) Setup() {
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/delivery/http/route/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/delivery/http/route/route.go internal/delivery/http/route/route_test.go
git commit -m "feat: add empty RouteConfig skeleton"
```

---

### Task 10: Composition root (`internal/config/app.go`)

**Files:**
- Create: `internal/config/app.go`
- Create: `internal/config/app_test.go`

**Interfaces:**
- Consumes: `*Config` (Task 2), `*logrus.Logger` (Task 3), `*validator.Validate` (Task 4), `*fiber.App` (Task 5), `*gorm.DB` (Task 6), `route.RouteConfig` (Task 9, imported as `terrion-backend/internal/delivery/http/route`).
- Produces: `type BootstrapConfig struct { DB *gorm.DB; App *fiber.App; Log *logrus.Logger; Validate *validator.Validate; Config *Config }` and `func Bootstrap(bootstrapConfig *BootstrapConfig)`. Consumed by Task 11 (`main.go`).

- [ ] **Step 1: Write the failing test**

Create `internal/config/app_test.go`:

```go
package config

import "testing"

func TestBootstrap_WiresRouteConfigWithoutPanicking(t *testing.T) {
	cfg := &Config{}
	cfg.App.Name = "test-app"

	app := NewFiber(cfg)
	log := NewLogger(cfg)
	validate := NewValidator()

	Bootstrap(&BootstrapConfig{
		DB:       nil,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestBootstrap -v`
Expected: FAIL — `BootstrapConfig`/`Bootstrap` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/config/app.go`:

```go
package config

import (
	"terrion-backend/internal/delivery/http/route"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type BootstrapConfig struct {
	DB       *gorm.DB
	App      *fiber.App
	Log      *logrus.Logger
	Validate *validator.Validate
	Config   *Config
}

// Bootstrap is the composition root: it wires repositories, use cases,
// controllers, and middleware together, then hands them to the route
// config. As domains are added, construct them here in that order
// (repository -> usecase -> controller) before calling routeConfig.Setup().
func Bootstrap(bootstrapConfig *BootstrapConfig) {
	routeConfig := route.RouteConfig{
		App: bootstrapConfig.App,
	}
	routeConfig.Setup()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestBootstrap -v`
Expected: PASS.

- [ ] **Step 5: Run the full config package test suite**

Run: `go test ./internal/config/... -v`
Expected: PASS for every test added in Tasks 2-6 and this task.

- [ ] **Step 6: Commit**

```bash
git add internal/config/app.go internal/config/app_test.go
git commit -m "feat: add Bootstrap composition root"
```

---

### Task 11: Entrypoint (`cmd/web/main.go`)

**Files:**
- Create: `cmd/web/main.go`

**Interfaces:**
- Consumes: `config.NewConfig()`, `config.NewLogger(cfg)`, `config.NewDatabase(cfg, log)`, `config.NewValidator()`, `config.NewFiber(cfg)`, `config.Bootstrap(&config.BootstrapConfig{...})` — all from Tasks 2-6 and 10.
- Produces: the running web server. Nothing else consumes this (terminal task for runnable code; Task 12 is documentation-only).

- [ ] **Step 1: Write `main.go`**

Create `cmd/web/main.go`:

```go
package main

import (
	"fmt"

	"terrion-backend/internal/config"
)

func main() {
	cfg := config.NewConfig()
	log := config.NewLogger(cfg)
	db := config.NewDatabase(cfg, log)
	validate := config.NewValidator()
	app := config.NewFiber(cfg)

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
	})

	err := app.Listen(fmt.Sprintf(":%d", cfg.Web.Port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
```

- [ ] **Step 2: Tidy and build the whole module**

Run: `go mod tidy`
Expected: no errors; `go.mod`/`go.sum` updated to reflect exactly the dependencies actually imported.

Run: `go build ./...`
Expected: builds successfully with no errors (this compiles every package written so far, including `cmd/web`).

Run: `go vet ./...`
Expected: no issues reported.

Run: `go test ./...`
Expected: PASS across `internal/config`, `internal/model`, `internal/repository`, `internal/delivery/http/route`.

- [ ] **Step 3: Note the manual runtime verification that requires real credentials**

`go build`/`go vet`/`go test` confirm the code compiles and the unit-testable pieces behave correctly, but actually starting the server and connecting to Supabase requires real credentials this session does not have. Document for the user: once `.env` is filled in with real `DB_HOST`/`DB_USER`/`DB_PASSWORD` from the Supabase dashboard, run:

```bash
cp .env.example .env
# edit .env with real Supabase credentials
go run cmd/web/main.go
```

Expected: logs a successful DB connection and `Listen`ing on the configured port with no `.env` present being handled gracefully (defaults apply, but `DB_HOST=""` will fail to connect — expected until real credentials are supplied).

- [ ] **Step 4: Commit**

```bash
git add cmd/web/main.go go.mod go.sum
git commit -m "feat: add cmd/web entrypoint wiring config, db, and bootstrap"
```

---

### Task 12: `PROJECT_STRUCTURE.md`

**Files:**
- Create: `PROJECT_STRUCTURE.md`

**Interfaces:**
- Consumes: the final structure and code from Tasks 1-11 (this task documents what exists after them).
- Produces: nothing consumed by code — this is the architecture reference for Claude Code and future contributors.

- [ ] **Step 1: Write `PROJECT_STRUCTURE.md`**

Create `PROJECT_STRUCTURE.md` at the repo root with these sections (write full content, not a summary):

```markdown
# Terrion Backend — Project Structure

## Overview

Terrion Backend follows a layered clean architecture adapted from
[khannedy/golang-clean-architecture](https://github.com/khannedy/golang-clean-architecture),
with two deliberate differences from that reference: there is no Kafka /
worker process (HTTP only), and configuration comes from `.env` instead of
`config.json` + Viper. A request flows delivery -> usecase -> repository ->
entity, and the response flows back the other way through a converter.

## Folder-by-folder reference

- `cmd/web/main.go` — process entrypoint. Builds config, logger, DB,
  validator, and the Fiber app, then calls `config.Bootstrap()`.
- `internal/config/` — the composition root and all infrastructure
  constructors:
  - `env.go` — `Config` struct + `NewConfig()`, loads `.env` via godotenv
    then reads typed values from the environment.
  - `logrus.go` — `NewLogger(cfg)`.
  - `validator.go` — `NewValidator()`.
  - `fiber.go` — `NewFiber(cfg)` + the JSON error handler.
  - `gorm.go` — `NewDatabase(cfg, log)`, connects to Supabase Postgres.
  - `app.go` — `BootstrapConfig` + `Bootstrap()`, the composition root that
    wires repository -> usecase -> controller -> route for every domain.
- `internal/constants/` — shared constants used across domains: enums,
  status codes, message strings, context keys. Empty until the first domain
  needs a shared constant; domain-specific constants that aren't reused
  belong in that domain's own `model` file instead.
- `internal/delivery/http/` — the HTTP layer:
  - `<domain>_controller.go` (one per domain, added at the package root
    alongside `route/` and `middleware/`) — parses requests into
    `model.*Request` structs, calls a usecase, wraps the result in
    `model.WebResponse[T]`.
  - `middleware/` — Fiber middleware (e.g. auth), following the reference's
    pattern of a `New<Name>(...) fiber.Handler` constructor.
  - `route/route.go` — `RouteConfig{App *fiber.App, <Domain>Controller
    *http.<Domain>Controller, ...}` + `Setup()`, registers every route.
- `internal/entity/` — GORM-mapped structs, one file per entity
  (`<domain>_entity.go`), with `gorm:"column:..."` tags and a `TableName()`
  method.
- `internal/model/` — request/response DTOs:
  - `model.go` — shared envelope types: `WebResponse[T]`, `PageResponse[T]`,
    `PageMetadata`. Every controller response is wrapped in `WebResponse[T]`.
  - `<domain>_model.go` — per-domain request/response structs with
    `validate:"..."` tags.
  - `converter/<domain>_converter.go` — pure functions mapping
    `entity.<Domain>` to `model.<Domain>Response` (and back where needed).
- `internal/repository/` — data access:
  - `repository.go` — generic `Repository[T]` base (`Create`, `Update`,
    `Delete`, `CountById`, `FindById`), takes `*gorm.DB` per call.
  - `<domain>_repository.go` — embeds `Repository[entity.<Domain>]`, adds
    domain-specific queries.
- `internal/usecase/` — business logic, one file per domain
  (`<domain>_usecase.go`). **Owns the transaction**: opens
  `db.WithContext(ctx).Begin()`, `defer tx.Rollback()`, validates the
  request via the injected `*validator.Validate`, calls repository methods
  passing `tx`, commits, then converts the result to a response model. This
  is why repositories take `db *gorm.DB` as a parameter instead of holding
  their own DB field — the usecase decides the transaction boundary.
- `db/migrations/` — SQL migrations, `golang-migrate` naming convention:
  `<timestamp>_<description>.up.sql` / `.down.sql`. Not yet wired to a
  migration runner in this scaffold; add `golang-migrate/migrate` when the
  first migration is written.

## Request lifecycle

1. HTTP request hits a controller in `internal/delivery/http/`.
2. The controller parses the body into a `model.*Request` struct
   (`ctx.BodyParser(request)`), and calls a usecase method with
   `ctx.UserContext()`.
3. The usecase begins a transaction, validates the request, builds/loads
   `entity.*` structs, and calls repository methods passing the
   transaction's `*gorm.DB`.
4. The repository runs GORM calls against the entity and returns.
5. The usecase commits the transaction, converts the entity to a response
   model via `internal/model/converter`, and returns it.
6. The controller wraps the response in `model.WebResponse[T]{Data:
   response}` and returns it as JSON. Errors are `*fiber.Error` values
   (e.g. `fiber.ErrBadRequest`), rendered by the error handler registered
   in `config.NewFiber`.

## Configuration

All configuration is environment variables, loaded from a `.env` file (via
`godotenv.Load()`, ignored if absent) into a typed `*config.Config` struct
in `internal/config/env.go`. See `.env.example` for every key, its default,
and (for the `DB_*` keys) where to find the value in the Supabase dashboard.

To add a new config key: add the field to the appropriate nested struct in
`Config`, read it in `NewConfig()` with `getEnv`/`getEnvAsInt`/
`getEnvAsBool`, and document it in `.env.example`.

## Dependency injection / composition root

`internal/config/app.go`'s `Bootstrap()` function is where every domain's
repository, usecase, and controller get constructed and wired together, in
that order, before being handed to `route.RouteConfig.Setup()`. When adding
a new domain, extend `Bootstrap()` (and `BootstrapConfig` if the domain
needs something not already passed in) rather than constructing dependencies
anywhere else.

## Constants convention

`internal/constants/` holds constants shared across more than one domain:
enums (e.g. order status), HTTP header names, context keys, standard
message strings. A constant used by only one domain lives in that domain's
own file instead (e.g. as untyped consts near the top of
`<domain>_usecase.go` or `<domain>_model.go`).

## How to add a new domain — worked walkthrough (example: `Product`)

1. `internal/entity/product_entity.go` — `type Product struct { ID string
   \`gorm:"column:id;primaryKey"\`; Name string \`gorm:"column:name"\`; ... }`
   plus `func (p *Product) TableName() string { return "products" }`.
2. `internal/model/product_model.go` — `ProductResponse`,
   `CreateProductRequest`, `UpdateProductRequest`, etc., with
   `validate:"required,max=100"`-style tags.
3. `internal/model/converter/product_converter.go` — `func
   ProductToResponse(product *entity.Product) *model.ProductResponse`.
4. `internal/repository/product_repository.go` — `type ProductRepository
   struct { repository.Repository[entity.Product] }` + any custom queries.
5. `internal/usecase/product_usecase.go` — `type ProductUseCase struct { DB
   *gorm.DB; Log *logrus.Logger; Validate *validator.Validate;
   ProductRepository *repository.ProductRepository }`, with methods like
   `Create`, `Get`, `Update`, `Delete`, `List`, each owning its own
   transaction as described in "Request lifecycle" above.
6. `internal/delivery/http/product_controller.go` — `type ProductController
   struct { Log *logrus.Logger; UseCase *usecase.ProductUseCase }`, one
   method per usecase method, parsing requests and wrapping responses.
7. `internal/delivery/http/route/route.go` — add `ProductController
   *http.ProductController` to `RouteConfig`, register its routes in
   `Setup()`.
8. `internal/config/app.go` — in `Bootstrap()`, construct
   `productRepository`, then `productUseCase`, then `productController`, and
   pass `productController` into `routeConfig`.
9. `db/migrations/<timestamp>_create_table_products.up.sql` /
   `.down.sql` — the table `product_entity.go` maps to.

## What's intentionally absent

- **No Kafka, no `cmd/worker`, no `delivery/messaging` or
  `gateway/messaging`.** The reference repo has these for async
  event-driven flows; Terrion Backend is HTTP-only. Do not reintroduce them
  by copying patterns from the reference repo — if async messaging is
  needed later, it should be a deliberate new design decision, not a
  copy-paste.
- **No `config.json` / Viper.** Configuration is `.env`-only, via
  `internal/config/env.go`.
- **No business domain yet.** This scaffold is intentionally empty of
  entities/usecases/controllers — use the walkthrough above for the first
  one.

## Not yet present — add when needed

- **`api/api-spec.json`** — the reference repo keeps an API spec (IntelliJ
  HTTP Client / OpenAPI-style) documenting every endpoint. Add an `api/`
  folder with a spec file once there are endpoints worth documenting;
  there's no fixed format required, pick whatever the team standardizes on.
- **`test/` integration tests** — the reference repo has a `test/` package
  with an `init.go` that builds the full `Config`/`DB`/`Fiber` stack once
  via `func init()` and hits real endpoints against a real (test) database
  per domain (`user_test.go`, etc.). Follow that convention once the first
  domain exists: package-level `init()` wiring shared across test files,
  one `_test.go` per domain, run with `go test -v ./test/`.
```

- [ ] **Step 2: Verify**

Run: `git status --short`
Expected: shows `PROJECT_STRUCTURE.md` as untracked/new.

- [ ] **Step 3: Commit**

```bash
git add PROJECT_STRUCTURE.md
git commit -m "docs: add PROJECT_STRUCTURE.md architecture reference"
```
