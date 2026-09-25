# Changelog

## v1.2.0 — 2026-09-25

### Changed

- `raptor new` no longer sets `max_body_bytes: 0` in `.raptor.dev.yaml` and `.raptor.test.yaml`, which disabled Raptor's 8 MB request body limit. A comment explains when to raise it.
- `raptor db init postgres` no longer writes `password: postgres` into the tracked config files. A comment points to `DATABASE_PASSWORD`, and the command's "Next steps" output says to set it.
