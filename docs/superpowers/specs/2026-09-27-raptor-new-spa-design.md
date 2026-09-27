# `raptor new` without cross-origin CORS, and `--spa` — design

**Date:** 2026-09-27
**Repository:** `go-raptor/cli` (target release v1.4.0)
**Conventions it emits:**
- raptor-api-conventions: `SKILL.md` → *Serving the SPA*, and `references/auth.md` → *CSRFMiddleware*.
- sveltekit-conventions: `references/setup.md` → the Vite proxy.

Both are at commit `beb574b` of `h00s/claude-skills`.

## Goal

`raptor new` currently scaffolds a cross-origin development setup:
- It registers the `cors` middleware and sets `app.cors_allow_origins: "http://localhost:5173"` in `.raptor.dev.yaml`, so a Vite dev server on port 5173 can call the API directly.
- It registers no `csrf` middleware.
- It has no way to serve the frontend build.

The conventions say the opposite:
- The API and the SPA share one origin, in development (Vite proxies `/api`) and in production (Go serves the build with `controllers/spa`).
- `csrf` protects every write, and there is no CORS.

This change makes the template same-origin. Serving a frontend becomes an opt-in `--spa` flag.

**Success looks like:**
- `raptor new example.com/shop` creates an API-only project:
  - `logger` and `csrf` are registered, with no CORS and no SPA controller.
  - It compiles, passes `go vet`, and passes its own tests.
- `raptor new example.com/shop --spa` creates the same project plus the `controllers/spa` wiring, as teacher uses it:
  - It boots and passes its tests with no frontend build.
  - A production run without `public/` fails at startup.
- Neither project registers the `cors` middleware or sets `cors_allow_origins` in a config file. Only the README mentions them, as the way to add a cross-origin frontend.

## Decisions (from the design discussion)

| Question | Decision |
|---|---|
| Default | API-only. `--spa` opts into serving a frontend. |
| Cross-origin frontends | Neither variant registers CORS. The generated README says how to add it for a frontend that must live on another origin. |
| CSRF | Both variants register `csrf`. It protects cookie-authenticated writes and passes non-browser clients, because requests without `Origin` or `Sec-Fetch-Site` go through `http.CrossOriginProtection`. |
| Frontend scaffolding | Out of scope: no SvelteKit app and no Dockerfile. |
| How the variants differ | Only the files that vary become `text/template` files. Everything else stays a verbatim copy. |

## Command

```
raptor new <module> [--spa]
```

- `--spa`: "serve a single-page frontend from public/ with controllers/spa".
- The closing "Get started" output stays `cd <dir>` and `raptor dev`.
- With `--spa`, it adds one line: `Frontend: proxy /api to http://127.0.0.1:3000 in vite.config.ts, and copy its build into public/ for production (see README.md).`

## What each variant generates

Files not listed here are unchanged from today.

### Both variants

`config/components/middlewares.go` is a plain file, the same in both variants. `raptor.Use` replaces `core.Use`, matching the conventions, so the `core` import goes:

```go
package components

import (
	"github.com/go-raptor/middlewares/csrf"
	"github.com/go-raptor/middlewares/logger"
	"github.com/go-raptor/raptor/v4"
)

func Middlewares() raptor.Middlewares {
	return raptor.Middlewares{
		raptor.Use(&logger.LoggerMiddleware{}),
		// Rejects cross-origin writes from browsers. A frontend on another origin needs
		// app.csrf_trusted_origins and the cors middleware; see README.md.
		raptor.Use(&csrf.CSRFMiddleware{}),
	}
}
```

`.raptor.dev.yaml` loses its `app:` section, which held only `cors_allow_origins`.

### `--spa` adds

`config/components/controllers.go` registers the controller after `HelloController`:

```go
		&controllers.HelloController{},
		// Serves the frontend build in public/; see README.md.
		&spa.SPAController{},
```

The `github.com/go-raptor/controllers/spa/v2` import is added with it.

`config/routes.yaml` gets the two routes from the spa README. Both are pinned by that module's `routes_test.go`:

```yaml
routes:
  /api/v1:
    /hello:
      GET: Hello.Greet
      POST: Hello.AddGreetings
    /{path...}: Errors.NotFound
  /: SPA.Index
```

`.raptor.dev.yaml` and `.raptor.test.yaml` each end with:

```yaml
app:
  # Boot without a frontend build in public/. Never set this in a config production loads.
  spa_optional: "true"
```

`.gitignore` adds `/public/`. The build is copied in when the image is built and is never committed.

### README.md

**API-only** projects get a short *Frontend* section:
- `raptor new --spa` serves a frontend from the same origin.
- A frontend that must live on another origin needs three things: the `cors` middleware registered, its origin in `cors_allow_origins`, and the same origin in `csrf_trusted_origins`.

**`--spa`** projects get a *Frontend* section with two parts.

*Development:* run `raptor dev`, then Vite, and open Vite's URL. The proxy must use the object form:

```ts
// vite.config.ts
server: {
  // The object form: the string form turns on changeOrigin, which rewrites Host, and csrf
  // then rejects every write.
  proxy: { "/api": { target: "http://127.0.0.1:3000" } },
},
```

The target is `127.0.0.1`, not `localhost`: Node 17 and later try `::1` first for `localhost`, and the Go server listens on IPv4.

*Production:*
- Copy SvelteKit's `build/` into `public/`, for example `COPY --from=frontend /app/build ./public` in a Dockerfile.
- A missing build fails at startup on purpose.
- The build is read into memory at boot, so a new build needs a restart.

## Template mechanism

- Files named `*.tmpl` under `_template/` are rendered with `text/template` and written without the suffix. Every other file is copied verbatim, as today.
- The data is `struct{ SPA bool }`.
- The existing module-path replacement (`github.com/go-raptor/template` → the new module) runs after rendering.
- The files that become templates, and only those, are:
  - `config/components/controllers.go.tmpl`
  - `config/routes.yaml.tmpl`
  - `.raptor.dev.yaml.tmpl`
  - `.raptor.test.yaml.tmpl`
  - `.gitignore.tmpl`
  - `README.md.tmpl`
- `copyTemplateFiles(projectDir, moduleName string)` becomes `copyTemplateFiles(projectDir, moduleName string, data templateData)`. `newProject` reads `--spa` into `data.SPA`.
- A template that fails to parse or execute is a bug in the CLI. The tests render every template in both variants, so such a bug never ships.

## Testing

**Render tests** (`internal/new`) run `copyTemplateFiles` into temporary directories, once per variant.

Default variant:
- `middlewares.go` imports `csrf` and not `cors`.
- `controllers.go` doesn't mention `spa`.
- `routes.yaml` has no `SPA.Index`.
- No `.yaml` file sets `spa_optional` or `cors_allow_origins`.
- `.gitignore` has no `/public/`.

`--spa` variant:
- `controllers.go` registers `&spa.SPAController{}`.
- `routes.yaml` parses, and maps `/` to `SPA.Index` and `/api/v1/{path...}` to `Errors.NotFound`.
- `.raptor.dev.yaml` and `.raptor.test.yaml` parse, with `app.spa_optional == "true"`.
- `.gitignore` has `/public/`.

Both variants:
- Every generated `.yaml` parses.
- No `.tmpl` file and no `{{` is left in the output.
- The existing body-limit test runs over both variants.

**Build test.** It is skipped under `-short` or when modules can't be downloaded, like `internal/generate/resource`'s `compileEnv`. For each variant it:
- runs the real generation (template, `go.mod`, `go mod tidy`) into a temporary directory;
- runs `go vet ./...` and `go test ./...` inside the generated project.

The `--spa` run proves the whole chain: the SPA controller boots with no `public/` because `.raptor.test.yaml` sets `spa_optional`. The template's hello tests POST without an `Origin` header, so `csrf` lets them through.

## Documentation and skills

- **CLI `README.md`:** a new *Creating a project* section covering:
  - `raptor new <module>`: API-only, same-origin, `csrf`.
  - `--spa`.
  - How to add CORS for a frontend on another origin.
- **CLI `CHANGELOG.md`, v1.4.0:**
  - Changed (behavior): new projects no longer register CORS or set `cors_allow_origins`, and they register `csrf`.
  - Added: `--spa`.
- **raptor-api-conventions `SKILL.md`:** the scaffolding list gains `raptor new <module> --spa` (CLI v1.4.0+) as how a new app starts.
- **sveltekit-conventions `references/setup.md`:**
  - Mention `raptor new --spa` for the backend.
  - The production paragraph says the spa/v2 controller "serves `public/`" instead of quoting `Directory: "public"`.

## Release

- CLI v1.4.0: `main` to origin and gh, the tag to gh only.
- The skills repository: one commit to origin.
- Both wait for an explicit go-ahead.

## Out of scope

- Scaffolding the SvelteKit app or a Dockerfile (approach C in the discussion).
- `raptor dev` also starting Vite.
- Converting existing projects. Their `.raptor.dev.yaml` and middlewares stay as generated.
- Putting the auth stack in the template.

## Risks

- **Muscle memory.** Anyone who used `raptor new` with Vite on 5173 calling the API directly now gets CORS errors. The CHANGELOG and both READMEs point to the Vite proxy, or to adding `cors`.
- **A proxy that rewrites `Host` breaks writes under `csrf`.** The README's Vite snippet uses the object form, with the reason in a comment.
- **Literal `{{` in a future template edit breaks rendering.** The render tests fail on it before release.
- **`go mod tidy` also resolves `csrf`,** plus `controllers/spa` with `--spa`, from the network. That's the same dependence `raptor new` already has on `raptor/v4`.
