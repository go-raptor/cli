# Changelog

## v1.5.2 — 2026-10-01

### Changed

- The generated `DatabaseService.HandleError` answers a cancelled context (shutdown cancelled the app context, or the client went away) with a 503 logged at debug, instead of an "Unhandled database error" at error level and a 500. A deadline still logs as an error.

## v1.5.1 — 2026-09-30

### Changed

- The `TestResponseHeaders` test that `raptor new` generates checks `/healthz` instead of the Hello sample, so deleting the sample doesn't break it.
- The READMEs say which log lines carry `request_id`: the request log and Raptor's own error lines, not the app's own, which read it with `ctx.Get(requestid.Key)` or `requestid.FromContext`.

## v1.5.0 — 2026-09-30

### Changed

- `raptor g resource` generates code for raptor v4.6.0: controllers bind with `ctx.Bind` (which answers 415 for a body not declared JSON, 400 for malformed JSON) and parse ids with `ctx.ParamInt64` and `ctx.QueryInt64`; `DatabaseService.Setup` sets `Ctx` to `s.AppContext()`, which is cancelled at shutdown; the tests use `raptor.JSONBody`, `raptor.DecodeJSON` and `raptor.WithCookie`. The generated `helpers.go` keeps only `validationFailed`, and the harness no longer defines `withSession`.
- A child resource's tests also check that a malformed parent filter (`?courseId=abc`) is a 400.

### Added

- `raptor new` registers the `requestid` and `secure` middlewares (order: requestid, logger, secure, csrf), routes `GET /healthz` and `GET /readyz` to Raptor's built-in health controller with a test for both, and documents `server.shutdown_delay` and `app.secure_hsts_max_age` in the generated README.

### Fixed

- Reading a module's version from `go.mod` (for the raptor floor and the spa/v2 check) now looks only at `require` directives. A `replace` or `exclude` block that named the module before the `require` block used to stand in for its version.

### Upgrading

- `raptor g resource` now requires `github.com/go-raptor/raptor/v4` v4.6.0 or later, and says so with the `go get` command to run. Existing `bindJSON`, `pathID` and `withSession` helpers keep compiling and can be deleted once nothing calls them. An `app/controllers/helpers.go` that doesn't declare `validationFailed` is reported, since the generated one can't be written beside it.

## v1.4.0 — 2026-09-27

### Changed

- **Behavior:** `raptor new` no longer registers the `cors` middleware or sets `cors_allow_origins: "http://localhost:5173"`, a cross-origin development setup. New projects are same-origin and register the `csrf` middleware instead. Proxy the frontend's dev server to the API (the README's *Creating a project* shows the Vite config), or add `cors` back for a frontend that must live on another origin.
- Generated Go files are gofmt-clean: imports are sorted after the module path is filled in.

### Added

- `raptor new --spa` serves a single-page frontend from `public/` with `controllers/spa`. It adds the `SPA.Index` catch-all route, `spa_optional` in `.raptor.dev.yaml` and `.raptor.test.yaml`, `/public/` in `.gitignore`, and README instructions for the Vite proxy and production.

## v1.3.1 — 2026-09-26

### Fixed

- The `TestMain` that `raptor g resource` generates no longer sets `SPA_OPTIONAL`, which `controllers/spa` v2.1.0 doesn't read. When the project requires `controllers/spa/v2`, the generator adds `spa_optional: "true"` under `app:` in `.raptor.test.yaml` instead, leaving a value the file already sets alone. For a spa/v2 older than v2.1.0, which ignores the setting, it says to upgrade.

## v1.3.0 — 2026-09-26

### Added

- `raptor g resource <Name> [field:type ...] [--parent Model] [--movable] [--plural Name]` scaffolds a complete entity (model, DTOs, zog schema, service with ownership checks, controller, routes, migration and integration tests), bootstrapping the shared services, helpers and test harness on first use. See the README.

## v1.2.0 — 2026-09-25

### Changed

- `raptor new` no longer sets `max_body_bytes: 0` in `.raptor.dev.yaml` and `.raptor.test.yaml`, which disabled Raptor's 8 MB request body limit. A comment explains when to raise it.
- `raptor db init postgres` no longer writes `password: postgres` into the tracked config files. A comment points to `DATABASE_PASSWORD`, and the command's "Next steps" output says to set it.
