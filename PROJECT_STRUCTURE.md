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

## Running the project

```bash
cp .env.example .env
# edit .env with real Supabase credentials (Project Settings -> Database)
go run cmd/web/main.go
```

Run the full test suite: `go test ./...`
