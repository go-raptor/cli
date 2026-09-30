![Raptor](https://static.husak.me/img/raptor/logo.png)

# Raptor CLI

This is the CLI for Raptor. It is heavily inspired by Buffalo CLI, which is a web development eco-system, designed to make the life of a web developer easier. With hot-reload, generators, and other tools, it is a full-stack web development environment.

## Installation

To install the Raptor CLI you can run the following command:

```bash
go install github.com/go-raptor/cli/cmd/raptor@latest
```

## Creating a project

```bash
raptor new github.com/you/shop
```

creates an API-only project in `shop/`. It is same-origin: there is no CORS, and the csrf middleware (`github.com/go-raptor/middlewares/csrf`) rejects cross-origin writes from browsers while letting other clients through.

Every project also registers `requestid` (an `X-Request-Id` on every response, and a `request_id` on the request log and Raptor's own error lines) and `secure` (security headers), and routes `GET /healthz` and `GET /readyz` to Raptor's built-in health controller. The generated README lists the production settings: `server.shutdown_delay`, so readiness fails before the listener closes, and `app.secure_hsts_max_age`, which turns on HSTS.

`--spa` also serves a single-page frontend (a SvelteKit `adapter-static` build, or any equivalent) from `public/` with `controllers/spa`, on the same origin:

- `SPA.Index` is routed on `/`, and `/api/v1/{path...}` keeps Raptor's JSON 404.
- `.raptor.dev.yaml` and `.raptor.test.yaml` set `spa_optional: "true"`, so the project boots and its tests pass before a frontend is built. `.raptor.dev.yaml` is for development only: production is configured with `.raptor.prod.yaml` or environment variables, which leave `spa_optional` unset, so a missing build fails at startup.
- The generated README covers development and production.

In development, either kind of project stays same-origin when the frontend's dev server proxies `/api` to it. With Vite, use the object form, so csrf sees the original `Host`:

```ts
// vite.config.ts
server: {
  proxy: { "/api": { target: "http://localhost:3000" } },
},
```

A frontend that must live on another origin needs the `cors` middleware registered, its origin in `app.cors_allow_origins`, and the same origin in `app.csrf_trusted_origins`.

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
| `name:string`, `code:string:12` | `VARCHAR(n) NOT NULL`, 150 by default | Required and trimmed |
| `notes:text` | `TEXT NOT NULL DEFAULT ''` | Prose; always optional, so it takes no `:optional` |
| `count:int`, `size:int64` | `INTEGER` / `BIGINT NOT NULL DEFAULT 0` with `CHECK (>= 0)` | 0 is valid |
| `active:bool` | `BOOLEAN NOT NULL DEFAULT false` | |
| `starts_at:time` | `TIMESTAMPTZ NOT NULL` | |
| `type:enum:lecture,lab` | a Postgres enum, `NOT NULL` | A typed Go string with constants and an allow-list |
| `division:ref`, `mentor:ref:User` | `division_id BIGINT NOT NULL` + FK `RESTRICT` | Shared reference data only; for a user's own rows use `--parent` |

Append `:optional` to any type but `text` (`starts_at:time:optional`) for a nullable column and an optional request field; an optional string stores `""` as NULL.

- A resource is owned by the user (`user_id`). `--parent Model` makes it a child at any depth: the generator follows the models' foreign keys up to `user_id` and writes the ownership checks. `--movable` lets an update move a child to another parent.
- A `ref` gets no ownership check, so its target must be shared reference data. The command refuses a target that is owned (a `user_id` of its own or from an embedded struct), that references users under another column name (`owner_id`), that reaches either through its foreign keys, or that it can't fully read: a struct embedded from another package (including a `BaseModel` that isn't `bun.BaseModel`), or a belongs-to relation to a type outside `app/models`. It follows belongs-to relations, with or without `join:`, and `x_id` columns named after a model; any other `x_id` column is not followed.
- It needs raptor v4.6.0 or later, the Bun Postgres connector (`raptor db init postgres --bun`) and the session auth stack (`models.User`, `AuthService.CurrentUser`). The generated code binds with `ctx.Bind`, parses ids with `ctx.ParamInt64` and `ctx.QueryInt64`, runs queries on `DatabaseService.Ctx` (the app's context, cancelled at shutdown), and its tests use `raptor.JSONBody`, `raptor.DecodeJSON` and `raptor.WithCookie`. On first use it creates `DatabaseService`, `ValidationService`, the controller helpers (`validationFailed`) and the test harness. Helpers from earlier versions (`bindJSON`, `pathID`, `withSession`) can stay; the new code doesn't call them. When the app serves its frontend with `controllers/spa`, the harness's `TestMain` boots it without a build: the generator adds `spa_optional: "true"` under `app:` in `.raptor.test.yaml`, which needs spa v2.1.0 or later.
- It never overwrites a file. An edit it can't make safely (a registration, `routes.yaml`) is printed for you to add by hand.
