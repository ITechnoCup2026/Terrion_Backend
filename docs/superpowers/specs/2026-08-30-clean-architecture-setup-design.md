# Terrion Backend — Clean Architecture Setup

Date: 2026-08-30
Status: Approved

## Purpose

Set up the initial project structure for `Terrion_Backend` (a Go backend, currently
empty except for `LICENSE`) following the layered clean architecture demonstrated
in [khannedy/golang-clean-architecture](https://github.com/khannedy/golang-clean-architecture),
adapted to this project's constraints:

- No Kafka, no background worker (`cmd/worker`, `delivery/messaging`,
  `gateway/messaging` are dropped entirely).
- Configuration via `.env` instead of `config.json` + Viper.
- Database is Supabase Postgres (session pooler / direct connection), not MySQL.
- A new `internal/constants/` folder for shared constants, created empty.
- Bare skeleton only — no example business domain (no User/auth vertical slice).
  The doc below substitutes for that example by walking through how to add one.

The reference repo was cloned and read in full (`cmd/web/main.go`,
`internal/config/*`, `internal/entity/user_entity.go`, `internal/repository/*`,
`internal/usecase/user_usecase.go`, `internal/model/*`, `internal/delivery/http/*`,
`go.mod`, `db/migrations/*`) before this design was drafted, so the mapping below
is based on the actual code, not the README summary.

## Reference architecture (as read from source)

Request lifecycle, traced from the reference's `user_usecase.go` / `user_controller.go` / `user_repository.go`:

1. `cmd/web/main.go` builds config, logger, DB, validator, Fiber app, then calls
   `config.Bootstrap()`.
2. `config.Bootstrap()` is the composition root: it constructs repositories, then
   usecases (injecting DB/log/validate/repository/producer), then controllers
   (injecting usecase/log), then middleware, then hands everything to
   `route.RouteConfig.Setup()`.
3. HTTP request hits a controller (`internal/delivery/http/*_controller.go`),
   which parses the request body into a `model.*Request` struct and calls the
   usecase, passing `ctx.UserContext()`.
4. The usecase (`internal/usecase/*_usecase.go`) **owns the transaction**: it
   opens `db.WithContext(ctx).Begin()`, `defer tx.Rollback()`, validates the
   request struct via `validator.Validate`, builds/mutates `entity.*` structs,
   calls repository methods passing the `tx`, and commits at the end. This is
   why repository methods take `db *gorm.DB` as a parameter instead of holding
   a DB field — the usecase decides the transaction boundary, not the
   repository.
5. Repositories (`internal/repository/*_repository.go`) embed a generic
   `Repository[T]` (`Create`, `Update`, `Delete`, `CountById`, `FindById` — all
   generic over the entity type via Go generics) and add entity-specific query
   methods (e.g. `FindByToken`). They only do gorm calls against `entity.*`
   structs; no business logic.
6. `entity.*` structs are the GORM-mapped DB rows (`gorm:"column:..."` tags,
   `TableName()` method).
7. The usecase converts `entity.*` → `model.*Response` via
   `internal/model/converter/*_converter.go` before returning to the
   controller, which wraps it in `model.WebResponse[T]{Data: response}` and
   returns it as JSON. Errors are returned as `*fiber.Error` values (e.g.
   `fiber.ErrBadRequest`) and rendered by a Fiber `ErrorHandler` registered in
   `config.NewFiber`.
8. Auth middleware (`delivery/http/middleware/auth_middleware.go`) calls
   `userUseCase.Verify()` per-request, stores the resulting `model.Auth` in
   `ctx.Locals("auth")`, retrieved via `middleware.GetUser(ctx)`.
9. Kafka producers (`gateway/messaging/*_producer.go`) are called from inside
   usecases, guarded by `if c.XProducer != nil` (config-driven enable/disable).
   Kafka consumers (`delivery/messaging/*_consumer.go`) run in `cmd/worker/main.go`
   as a separate process, calling usecases the same way controllers do.

Config today: single `config.json`, loaded by `config.NewViper()` (dotted keys
like `database.pool.idle`), consumed by `config.NewDatabase`, `config.NewFiber`,
`config.NewLogger`.

## Target structure for Terrion_Backend

```
Terrion_Backend/
├── cmd/
│   └── web/
│       └── main.go                  # entrypoint: build config/log/db/validator/fiber, call config.Bootstrap()
├── internal/
│   ├── config/
│   │   ├── env.go                   # godotenv.Load() + typed *Config struct (replaces viper.go)
│   │   ├── app.go                   # BootstrapConfig struct + Bootstrap() composition root
│   │   ├── fiber.go                 # NewFiber() + Fiber ErrorHandler
│   │   ├── gorm.go                  # NewDatabase(): postgres driver, Supabase DSN, pool settings
│   │   └── logrus.go                # NewLogger()
│   ├── constants/                   # empty — user populates as domains are added
│   ├── delivery/
│   │   └── http/
│   │       ├── middleware/          # empty — auth middleware added when first domain needs it
│   │       └── route/
│   │           └── route.go         # RouteConfig{App *fiber.App} + Setup() (no-op until first controller)
│   ├── entity/                      # empty — GORM structs go here per domain
│   ├── model/
│   │   ├── converter/               # empty — entity<->model mapping per domain
│   │   └── model.go                 # WebResponse[T], PageResponse[T], PageMetadata (framework-level, kept)
│   ├── repository/
│   │   └── repository.go            # generic Repository[T] base (framework-level, kept)
│   └── usecase/                     # empty — business logic per domain
├── db/
│   └── migrations/                  # empty — golang-migrate convention (see doc for naming)
├── .env.example                     # documents every config key, with Supabase-specific notes
├── .gitignore                       # includes .env
├── go.mod
├── go.sum
├── LICENSE                          # existing, untouched
└── PROJECT_STRUCTURE.md             # full architecture explanation for Claude Code / future contributors
```

No `internal/config/validator.go` — `go-playground/validator` is initialized
directly as `validator.New()`, but since it's a one-liner with no config
dependency it can live as a function in `app.go` or its own `validator.go`
file (implementation detail, decide during implementation — keep as its own
file to match the reference's one-file-per-concern convention).

### Dropped from the reference

- `cmd/worker/`
- `internal/delivery/messaging/` (consumers)
- `internal/gateway/messaging/` (producers)
- `internal/config/kafka.go`
- `internal/config/viper.go` (replaced by `env.go`)
- `config.json` (replaced by `.env` / `.env.example`)
- `BootstrapConfig.Producer` field and all producer wiring in `Bootstrap()`
- Kafka/Sarama, Viper dependencies from `go.mod`

### Skipped for now (documented as optional, not generated)

- `api/api-spec.json` — API spec convention noted in `PROJECT_STRUCTURE.md` for
  when there are endpoints to document.
- `test/` integration test scaffolding (`init.go`, `helper_test.go` pattern) —
  noted in the doc as the convention to follow once a domain exists to test.

## Config design

`internal/config/env.go`:

```go
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
        Host, User, Password, Name, SSLMode string
        Port                                int
        PoolIdle, PoolMax, PoolLifetime     int
    }
}

func NewConfig() *Config { ... }
```

Loaded via `godotenv.Load()` (error ignored — absent `.env` is normal in
environments that inject real env vars) then populated field-by-field from
`os.Getenv` with small `getEnv`/`getEnvAsInt`/`getEnvAsBool` helpers and
sensible defaults (`WEB_PORT` defaults to `8080`, `LOG_LEVEL` to logrus'
Info level, etc).

Env keys (documented in `.env.example`):

```
APP_NAME=terrion-backend
APP_ENV=development

WEB_PORT=8080
WEB_PREFORK=false

LOG_LEVEL=4

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

`.env.example` includes a comment block pointing at Supabase's Project
Settings → Database page for `DB_HOST`/`DB_USER`/`DB_PASSWORD`, and notes this
targets the session pooler / direct connection (port 5432, not the 6543
transaction pooler), so GORM's default prepared-statement caching is safe to
leave on.

`internal/config/gorm.go` builds the DSN with `gorm.io/driver/postgres`:

```go
dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
    cfg.Database.Host, cfg.Database.User, cfg.Database.Password,
    cfg.Database.Name, cfg.Database.Port, cfg.Database.SSLMode)
```

with the same `logrusWriter` + pool-setting pattern as the reference's
`gorm.go` (idle/max/lifetime from config), just on `*gorm.DB`/postgres instead
of mysql.

## Dependencies (go.mod)

Kept from reference: `github.com/gofiber/fiber/v2`, `github.com/go-playground/validator/v10`,
`github.com/sirupsen/logrus`, `gorm.io/gorm`.

Swapped: `gorm.io/driver/mysql` → `gorm.io/driver/postgres`; `github.com/spf13/viper` →
`github.com/joho/godotenv`.

Dropped: `github.com/IBM/sarama`.

Kept conditionally: `github.com/google/uuid`, `golang.org/x/crypto` (bcrypt) —
these are only used by the reference's User example. Since we're not
generating an example domain, they are **not** added to `go.mod` now; the
architecture doc's walkthrough will name them as the expected choice when the
first auth-style domain is added.

## `PROJECT_STRUCTURE.md` contents

Written for Claude Code (and future contributors) to understand the project
without re-deriving it from scratch. Sections:

1. **Overview** — one paragraph: layered clean architecture, request lifecycle
   summary.
2. **Folder-by-folder reference** — every folder in the target tree above,
   what belongs in it, and the naming convention for files in it
   (`xxx_entity.go`, `xxx_model.go`, `xxx_converter.go`, `xxx_repository.go`,
   `xxx_usecase.go`, `xxx_controller.go`).
3. **Request lifecycle** — the 9-step flow traced above (delivery → usecase →
   repository → entity, and back via converter), including who owns the
   transaction and why (usecase, not repository).
4. **Configuration** — `.env` keys, how `Config` is loaded and threaded
   through `Bootstrap()`, how to add a new key.
5. **Dependency injection / composition root** — how `internal/config/app.go`
   wires repository → usecase → controller → route, and where to add new
   wiring when adding a domain.
6. **Constants convention** — what belongs in `internal/constants/` (shared
   enums, status codes, message strings, context keys) vs. what belongs in a
   domain's own model file.
7. **How to add a new domain — worked walkthrough** — using a hypothetical
   `Product` domain, the exact sequence of files to create (entity → model →
   converter → repository → usecase → controller → route → migration) mirroring
   the reference's User feature, since there's no in-repo example to copy from.
8. **What's intentionally absent** — no Kafka, no worker process, no
   `config.json`/Viper — and why, so nobody re-introduces them by copying
   patterns from the reference repo directly.

## Assumptions confirmed by user

- Go module path: `terrion-backend`.
- Go version: 1.25.
- Default web port: `8080`.
- constants folder created empty, no seed content.
- Bare skeleton, no example domain.
- Supabase connection via session pooler / direct connection (port 5432),
  prepared statements left enabled.

## Out of scope

- Actually provisioning a Supabase project or running any migration against
  it — this task only sets up the connection *configuration* shape.
- CI/CD, Docker, Makefile — not requested.
- Any business domain/entity.
