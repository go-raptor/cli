![Raptor](https://static.husak.me/img/raptor/logo.png)

# Raptor CLI

This is the CLI for Raptor. It is heavily inspired by Buffalo CLI, which is a web development eco-system, designed to make the life of a web developer easier. With hot-reload, generators, and other tools, it is a full-stack web development environment.

## Installation

To install the Raptor CLI you can run the following command:

```bash
go install github.com/go-raptor/cli/cmd/raptor@latest
```

## Generated configuration

- **Request body limit.** New projects keep Raptor's 8 MB request body limit. Raise `server.max_body_bytes` only if you accept uploads, and still check each file's size in the upload endpoint.
- **Database password.** `raptor db init postgres` doesn't write a password into `.raptor.dev.yaml` or `.raptor.test.yaml`, because those files are tracked. Set `DATABASE_PASSWORD` in your environment instead.
- **Middleware registration.** `raptor g middleware` creates the middleware but doesn't register it, because its scope (`raptor.Use`, `UseOnly` or `UseExcept`) is your decision. Add it to `config/components/middlewares.go`.

## Generating a resource

`raptor g resource` scaffolds a whole entity the way the raptor-api-conventions conventions write one: the Bun model with its request and response DTOs, zog schema, `ToModel`/`ApplyTo` and update allowlist, a service with the ownership checks, the controller, routes, a Goose migration and integration tests.

```bash
raptor g resource Course name:string lecture_hours:int division:ref
raptor g resource Outcome name:string criteria:text position:int --parent Course
raptor g resource Unit title:string:80 type:enum:lecture,lab starts_at:time:optional --parent Outcome --movable
```

| Field spec | Column | Notes |
| --- | --- | --- |
| `name:string`, `code:string:12` | `VARCHAR(n)`, 150 by default | Required and trimmed; `:optional` stores `""` as NULL |
| `notes:text` | `TEXT NOT NULL DEFAULT ''` | Prose; always optional |
| `count:int`, `size:int64` | `INTEGER` / `BIGINT` with `CHECK (>= 0)` | 0 is valid |
| `active:bool` | `BOOLEAN NOT NULL DEFAULT false` | |
| `starts_at:time` | `TIMESTAMPTZ` | |
| `type:enum:lecture,lab` | a Postgres enum | A typed Go string with constants and an allow-list |
| `division:ref`, `mentor:ref:User` | `division_id BIGINT` + FK `RESTRICT` | Shared reference data only; for a user's own rows use `--parent` |

- A resource is owned by the user (`user_id`). `--parent Model` makes it a child at any depth: the generator follows the models' foreign keys up to `user_id` and writes the ownership checks. `--movable` lets an update move a child to another parent.
- It needs the Bun Postgres connector (`raptor db init postgres --bun`) and the session auth stack (`models.User`, `AuthService.CurrentUser`). On first use it creates `DatabaseService`, `ValidationService`, the controller helpers and the test harness.
- It never overwrites a file. An edit it can't make safely (a registration, `routes.yaml`) is printed for you to add by hand.
