# Changelog

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
