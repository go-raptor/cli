# `raptor new` same-origin template and `--spa` — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `raptor new` scaffold a same-origin, csrf-protected, API-only project by default, and serve a single-page frontend with `controllers/spa` behind an opt-in `--spa` flag.

**Architecture:**
- The template in `internal/new/_template` stays a tree of real files that are copied verbatim.
- The six files that differ between the variants become `*.tmpl` files, rendered with `text/template` and the data `templateData{SPA bool}`.
- After rendering, the module path is filled in, and every generated `.go` file goes through `go/format`.
- A build test generates both variants for real, then runs `go vet` and `go test` inside each.

**Tech Stack:** Go 1.26 (the CLI module), `text/template`, `go/format`, cobra, `go.yaml.in/yaml/v3` (already a CLI dependency), `github.com/go-raptor/middlewares/csrf` v1.0+, and `github.com/go-raptor/controllers/spa/v2` v2.1+.

**Spec:** `docs/superpowers/specs/2026-09-27-raptor-new-spa-design.md`

## Global Constraints

- Work on branch `raptor-new-spa`, created from `main` in `go-raptor/cli`. Integrate it only in Task 6.
- The placeholder module path is `github.com/go-raptor/template` (`templateModule` in `internal/new/new.go`). It is replaced *after* a `.tmpl` file is rendered.
- The template data is exactly `type templateData struct{ SPA bool }`.
- Exactly these six files become `.tmpl`:
  - `config/components/controllers.go.tmpl`
  - `config/routes.yaml.tmpl`
  - `.raptor.dev.yaml.tmpl`
  - `.raptor.test.yaml.tmpl`
  - `.gitignore.tmpl`
  - `README.md.tmpl`
- Neither variant imports `github.com/go-raptor/middlewares/cors`, and neither sets `cors_allow_origins` in a YAML file.
- Both variants register `raptor.Use(&logger.LoggerMiddleware{})`, then `raptor.Use(&csrf.CSRFMiddleware{})`. Use `raptor.Use`, not `core.Use`.
- `spa_optional: "true"` appears only with `--spa`, and only in `.raptor.dev.yaml` and `.raptor.test.yaml`.
- The Vite proxy target is `http://localhost:3000`, in the object form `proxy: { "/api": { target: "http://localhost:3000" } }`.
- The `--spa` routes are exactly `/{path...}: Errors.NotFound` under `/api/v1`, and `/: SPA.Index` at the top level.
- Commit subjects are `new: <imperative>` for `internal/new`, and `docs: <imperative>` for documentation. Every commit message ends with:
  ```
  Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
  ```
- Nothing is pushed or tagged before Task 6, and Task 6 waits for an explicit go-ahead.
- The test command for `internal/new` is `go test -count=1 ./internal/new/`, run from the CLI repository root.

## Review Focus

1. **A `.tmpl` file whose module path is left unreplaced.** `controllers.go.tmpl` imports `github.com/go-raptor/template/app/controllers`, and the generated project must import `example.com/shop/app/controllers`. Pinned in Task 1: no generated file may contain `templateModule`.
2. **Import order after the module is filled in.** Once `example.com/shop/...` replaces `github.com/go-raptor/template/...`, the imports are no longer sorted, and a `{{if}}` inside an import block can leave a stray blank line. Every generated `.go` file should be gofmt-clean in both variants. Pinned in Task 1.
3. **Dotfiles rendered from `.tmpl`** (`.gitignore`, `.raptor.dev.yaml`, `.raptor.test.yaml`) must exist in both variants. The embed is `all:_template`, and a rename could drop them. Pinned in Task 1 through the `rendered` list.
4. **The `--spa` project keeps the JSON 404 for unknown API paths, and boots with no `public/`.** The template's own `TestNotFound` and `TestMain` exercise both, but only when the generated project's tests actually run. Pinned in Task 4 by the build test.
5. **The build test must not skip when the template itself is broken.** It first probes the module proxy with a module the template doesn't author (`csrf@latest`). After a successful probe, a `go mod tidy`, `go vet` or `go test` failure fails the test. Pinned in Task 4.

---

### Task 1: Render `.tmpl` files and format generated Go

**Files:**
- Modify: `internal/new/new.go`, where `copyTemplateFiles` gains a data parameter, plus `render` and `templateData`.
- Rename:
  - `internal/new/_template/config/components/controllers.go` → `controllers.go.tmpl`
  - `internal/new/_template/config/routes.yaml` → `routes.yaml.tmpl`
  - `internal/new/_template/.raptor.dev.yaml` → `.raptor.dev.yaml.tmpl`
  - `internal/new/_template/.raptor.test.yaml` → `.raptor.test.yaml.tmpl`
  - `internal/new/_template/.gitignore` → `.gitignore.tmpl`
  - `internal/new/_template/README.md` → `README.md.tmpl`
- Test: rewrite `internal/new/new_test.go`.

**Interfaces:**
- Produces:
  - `type templateData struct{ SPA bool }`
  - `func copyTemplateFiles(projectDir, moduleName string, data templateData) error`
  - `func render(name string, content []byte, data templateData) ([]byte, error)`
- Test helpers used by later tasks:
  - `func variants(t *testing.T) map[string]string`, which maps `"api"` and `"spa"` to rendered directories.
  - `func readFile(t *testing.T, dir, name string) string`

- [ ] **Step 0: Create the branch**

```bash
git switch -c raptor-new-spa
```

- [ ] **Step 1: Write the failing tests.** Replace the whole of `internal/new/new_test.go` with:

```go
package new

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// variants renders the template into a temporary directory per variant, keyed "api" and "spa".
func variants(t *testing.T) map[string]string {
	t.Helper()
	dirs := map[string]string{}
	for name, data := range map[string]templateData{"api": {}, "spa": {SPA: true}} {
		dir := t.TempDir()
		if err := copyTemplateFiles(dir, "example.com/shop", data); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		dirs[name] = dir
	}
	return dirs
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// rendered lists the files the .tmpl sources become, dotfiles included.
var rendered = []string{".gitignore", ".raptor.dev.yaml", ".raptor.test.yaml", "README.md", "config/routes.yaml", "config/components/controllers.go"}

// Both variants render completely: every .tmpl source becomes its file, no template action or
// placeholder module path is left, the YAML parses, and Go comes out gofmt-clean.
func TestTemplateRendersInBothVariants(t *testing.T) {
	for variant, dir := range variants(t) {
		for _, name := range rendered {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
				t.Errorf("%s: %s is missing", variant, name)
			}
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.HasSuffix(rel, ".tmpl") {
				t.Errorf("%s: %s was copied instead of rendered", variant, rel)
			}
			if bytes.Contains(b, []byte("{{")) {
				t.Errorf("%s: %s still holds a template action", variant, rel)
			}
			if bytes.Contains(b, []byte(templateModule)) {
				t.Errorf("%s: %s still names %s", variant, rel, templateModule)
			}
			if strings.HasSuffix(rel, ".yaml") {
				var v map[string]any
				if err := yaml.Unmarshal(b, &v); err != nil {
					t.Errorf("%s: %s does not parse: %v", variant, rel, err)
				}
			}
			if strings.HasSuffix(rel, ".go") {
				if formatted, err := format.Source(b); err != nil || !bytes.Equal(formatted, b) {
					t.Errorf("%s: %s is not gofmt-clean (%v)", variant, rel, err)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewProjectKeepsBodyLimit(t *testing.T) {
	disabled := regexp.MustCompile(`(?m)^\s*max_body_bytes:\s*0\s*$`)
	for variant, dir := range variants(t) {
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if b := readFile(t, dir, name); disabled.MatchString(b) {
				t.Errorf("%s: %s disables the request body limit:\n%s", variant, name, b)
			}
		}
	}
}
```

- [ ] **Step 2: Run the tests.**

Run: `go test -count=1 ./internal/new/`
Expected: a build failure, `too many arguments in call to copyTemplateFiles`.

- [ ] **Step 3: Add the parameter, with no behaviour yet.** In `internal/new/new.go`, above `copyTemplateFiles`, add:

```go
// templateData is what the *.tmpl files in _template are rendered with.
type templateData struct {
	SPA bool // serve a frontend build from public/ with controllers/spa
}
```

Change the signature to `func copyTemplateFiles(projectDir, moduleName string, data templateData) error`, and in `newProject` change the call to `copyTemplateFiles(projectDir, moduleName, templateData{})`.

Run: `go test -count=1 ./internal/new/`
Expected: FAIL. Among the lines are `api: config/components/controllers.go is not gofmt-clean` and `api: main.go is not gofmt-clean`. The imports that pointed at the placeholder module now sort before `github.com/...`.

- [ ] **Step 4: Rename the six files to `.tmpl`.** Their contents don't change in this task.

```bash
cd internal/new/_template
git mv config/components/controllers.go config/components/controllers.go.tmpl
git mv config/routes.yaml config/routes.yaml.tmpl
git mv .raptor.dev.yaml .raptor.dev.yaml.tmpl
git mv .raptor.test.yaml .raptor.test.yaml.tmpl
git mv .gitignore .gitignore.tmpl
git mv README.md README.md.tmpl
cd ../../..
```

Run: `go test -count=1 ./internal/new/`
Expected: FAIL, now with `.gitignore is missing` and `config/routes.yaml.tmpl was copied instead of rendered` as well.

- [ ] **Step 5: Render and format.** In `internal/new/new.go`, add `"bytes"`, `"go/format"` and `"text/template"` to the imports, and replace the body of `copyTemplateFiles` from `content, err := templateFS.ReadFile(path)` to the end of the walk callback with:

```go
		content, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}
		// A .tmpl file differs between variants: render it, and drop the suffix
		if name, ok := strings.CutSuffix(targetPath, ".tmpl"); ok {
			if content, err = render(path, content, data); err != nil {
				return err
			}
			targetPath = name
		}
		// Replace template module name with the actual module name
		output := []byte(strings.ReplaceAll(string(content), templateModule, moduleName))
		// The new module path reorders imports, and a rendered conditional can leave a blank line
		if strings.HasSuffix(targetPath, ".go") {
			if output, err = format.Source(output); err != nil {
				return fmt.Errorf("formatting %s: %w", relPath, err)
			}
		}
		return os.WriteFile(targetPath, output, 0644)
	})
}

// render executes one .tmpl file of the template with data.
func render(name string, content []byte, data templateData) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
```

- [ ] **Step 6: Run the tests.**

Run: `go test -count=1 ./internal/new/ && go vet ./internal/new/ && gofmt -l internal/new`
Expected: `ok`, with no vet output and no gofmt output.

- [ ] **Step 7: Commit**

```bash
git add internal/new
git commit -F - <<'EOF'
new: render the template's varying files as text/template

Six files of the project template become .tmpl files, rendered with
templateData{SPA bool} before the module path is filled in. Generated Go
files go through go/format, so their imports stay sorted once the module
path replaces the placeholder. Output is unchanged otherwise.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
```

---

### Task 2: Same-origin by default (csrf, no CORS)

**Files:**
- Modify: `internal/new/_template/config/components/middlewares.go`
- Modify: `internal/new/_template/.raptor.dev.yaml.tmpl`, dropping the `app:` section.
- Modify: `internal/new/_template/README.md.tmpl`, appending the API-only *Frontend* section.
- Test: `internal/new/new_test.go`

**Interfaces:**
- Consumes: `variants`, `readFile` (Task 1).
- Produces the test helper `func appConfig(t *testing.T, dir, name string) map[string]string`.

- [ ] **Step 1: Write the failing test.** Append to `internal/new/new_test.go`:

```go
// appConfig is the app: mapping of a generated config file.
func appConfig(t *testing.T, dir, name string) map[string]string {
	t.Helper()
	var cfg struct {
		App map[string]string `yaml:"app"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, dir, name)), &cfg); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return cfg.App
}

// Neither variant opens the API to other origins: csrf protects writes, and nothing registers
// cors or lists cors_allow_origins.
func TestNoVariantIsCrossOrigin(t *testing.T) {
	for variant, dir := range variants(t) {
		mw := readFile(t, dir, "config/components/middlewares.go")
		if !strings.Contains(mw, "raptor.Use(&csrf.CSRFMiddleware{})") {
			t.Errorf("%s: middlewares.go does not register csrf:\n%s", variant, mw)
		}
		if strings.Contains(mw, "middlewares/cors") {
			t.Errorf("%s: middlewares.go registers cors:\n%s", variant, mw)
		}
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if _, ok := appConfig(t, dir, name)["cors_allow_origins"]; ok {
				t.Errorf("%s: %s sets cors_allow_origins", variant, name)
			}
		}
	}
}
```

- [ ] **Step 2: Run the test.**

Run: `go test -count=1 -run TestNoVariantIsCrossOrigin ./internal/new/`
Expected: FAIL, including `api: middlewares.go does not register csrf` and `api: .raptor.dev.yaml sets cors_allow_origins`.

- [ ] **Step 3: Replace the middlewares.** `internal/new/_template/config/components/middlewares.go` becomes:

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

- [ ] **Step 4: Drop the CORS origin from the dev config.** In `internal/new/_template/.raptor.dev.yaml.tmpl`, delete the last three lines: the blank line, `app:`, and `  cors_allow_origins: "http://localhost:5173"`. The file now ends with `  ip_extractor: direct` and a newline.

- [ ] **Step 5: Document the API-only frontend options.** Append to `internal/new/_template/README.md.tmpl`, directly after the final line `**Response:** \`201 Created\``:

````text
{{- if not .SPA}}

## Frontend

The API is same-origin: there is no CORS, and the csrf middleware rejects cross-origin writes from browsers while letting other clients through. `raptor new <module> --spa` generates a project that also serves a single-page frontend from `public/` on the same origin.

A frontend that must live on another origin needs the `cors` middleware (`github.com/go-raptor/middlewares/cors`) registered in `config/components/middlewares.go`, its origin in `app.cors_allow_origins`, and the same origin in `app.csrf_trusted_origins`.
{{- end}}
````

The file must end with a newline after `{{- end}}`.

- [ ] **Step 6: Run the tests.**

Run: `go test -count=1 ./internal/new/`
Expected: `ok`. All tests pass, including Task 1's render test on the new README.

- [ ] **Step 7: Commit**

```bash
git add internal/new
git commit -F - <<'EOF'
new: generate a same-origin API with csrf instead of CORS

New projects registered cors and allowed http://localhost:5173, a
cross-origin development setup the conventions reject. They now register
the csrf middleware and no cors, and the README says how to add cors for
a frontend that must live on another origin.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
```

---

### Task 3: `--spa` serves the frontend

**Files:**
- Modify: `internal/new/new.go`, adding the `--spa` flag, passing it through, and printing the "Get started" line.
- Modify:
  - `internal/new/_template/config/components/controllers.go.tmpl`
  - `internal/new/_template/config/routes.yaml.tmpl`
  - `internal/new/_template/.raptor.dev.yaml.tmpl`
  - `internal/new/_template/.raptor.test.yaml.tmpl`
  - `internal/new/_template/.gitignore.tmpl`
  - `internal/new/_template/README.md.tmpl`
- Test: `internal/new/new_test.go`

**Interfaces:**
- Consumes: `variants`, `readFile`, `appConfig` (Tasks 1–2), and `Cmd` (`internal/new/new.go`).
- Produces the package variable `var spa bool`, bound to `--spa`.

- [ ] **Step 1: Write the failing tests.** Add `"slices"` to the imports of `internal/new/new_test.go`, and append:

```go
// routes is config/routes.yaml as the router reads it.
func routes(t *testing.T, dir string) map[string]any {
	t.Helper()
	var r struct {
		Routes map[string]any `yaml:"routes"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, dir, "config/routes.yaml")), &r); err != nil {
		t.Fatal(err)
	}
	return r.Routes
}

// --spa wires controllers/spa exactly as its README routes it, relaxes it only where there is no
// build (development and tests), and keeps the build out of git. The API-only default has none
// of it.
func TestSPAVariantServesTheFrontend(t *testing.T) {
	dirs := variants(t)
	for variant, want := range map[string]bool{"api": false, "spa": true} {
		dir := dirs[variant]
		controllers := readFile(t, dir, "config/components/controllers.go")
		if got := strings.Contains(controllers, "&spa.SPAController{}"); got != want {
			t.Errorf("%s: registers SPAController = %v, want %v:\n%s", variant, got, want, controllers)
		}
		r := routes(t, dir)
		api, _ := r["/api/v1"].(map[string]any)
		if got := r["/"] == "SPA.Index" && api["/{path...}"] == "Errors.NotFound"; got != want {
			t.Errorf("%s: routes / to SPA.Index and keeps the JSON 404 for /api/v1 = %v, want %v: %v", variant, got, want, r)
		}
		if api["/hello"] == nil {
			t.Errorf("%s: lost the /api/v1/hello routes: %v", variant, r)
		}
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if got := appConfig(t, dir, name)["spa_optional"] == "true"; got != want {
				t.Errorf("%s: %s sets spa_optional \"true\" = %v, want %v", variant, name, got, want)
			}
		}
		if got := slices.Contains(strings.Split(readFile(t, dir, ".gitignore"), "\n"), "/public/"); got != want {
			t.Errorf("%s: .gitignore ignores /public/ = %v, want %v", variant, got, want)
		}
	}
}

func TestSPAIsAnOptInFlag(t *testing.T) {
	if f := Cmd.Flags().Lookup("spa"); f == nil || f.DefValue != "false" {
		t.Fatalf("--spa must be a flag that defaults to off, got %+v", f)
	}
}
```

- [ ] **Step 2: Run the tests.**

Run: `go test -count=1 -run 'TestSPA' ./internal/new/`
Expected: FAIL. The failures include `spa: registers SPAController = false, want true` and `--spa must be a flag that defaults to off, got <nil>`.

- [ ] **Step 3: Register the SPA controller for `--spa`.** `internal/new/_template/config/components/controllers.go.tmpl` becomes:

```text
package components

import (
{{- if .SPA}}
	"github.com/go-raptor/controllers/spa/v2"
{{- end}}
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/app/controllers"
)

func Controllers() raptor.Controllers {
	return raptor.Controllers{
		&controllers.HelloController{},
{{- if .SPA}}
		// Serves the frontend build in public/; see README.md.
		&spa.SPAController{},
{{- end}}
	}
}
```

- [ ] **Step 4: Route the catch-all for `--spa`.** `internal/new/_template/config/routes.yaml.tmpl` becomes:

```text
routes:
  /api/v1:
    /hello:
      GET: Hello.Greet
      POST: Hello.AddGreetings
{{- if .SPA}}
    /{path...}: Errors.NotFound
  /: SPA.Index
{{- end}}
```

- [ ] **Step 5: Relax the build check in development and tests only.** Append to *both* `internal/new/_template/.raptor.dev.yaml.tmpl` and `internal/new/_template/.raptor.test.yaml.tmpl`, directly after their final `  ip_extractor: direct` line:

```text
{{- if .SPA}}

app:
  # Boot without a frontend build in public/. Never set this in a config production loads.
  spa_optional: "true"
{{- end}}
```

Each file must end with a newline after `{{- end}}`.

- [ ] **Step 6: Ignore the build.** `internal/new/_template/.gitignore.tmpl` becomes:

```text
/bin
{{- if .SPA}}
/public/
{{- end}}
```

It ends with a newline.

- [ ] **Step 7: Document the frontend for `--spa`.** In `internal/new/_template/README.md.tmpl`, replace the block that Task 2 appended (from `{{- if not .SPA}}` to `{{- end}}`) with:

````text
{{- if .SPA}}

## Frontend

The server also serves a single-page frontend (a SvelteKit `adapter-static` build, or any equivalent) from `public/`, on the same origin as the API. There is no CORS; the csrf middleware rejects cross-origin writes from browsers.

### Development

Run `raptor dev`, then the frontend's dev server, and open the frontend's URL. Vite proxies `/api` to the Go server:

```ts
// vite.config.ts
server: {
  // The object form: the string form turns on changeOrigin, which rewrites Host, and csrf
  // then rejects every write.
  proxy: { "/api": { target: "http://localhost:3000" } },
},
```

`.raptor.dev.yaml` and `.raptor.test.yaml` set `spa_optional: "true"`, so the server boots and the tests pass before any frontend is built; every page returns 404 until one is.

### Production

Copy the frontend build into `public/`, for example `COPY --from=frontend /app/build ./public` in a Dockerfile. Production must not load `spa_optional`: a missing build then fails at startup instead of serving 404s. The build is read into memory at boot, so a new build needs a restart.
{{- else}}

## Frontend

The API is same-origin: there is no CORS, and the csrf middleware rejects cross-origin writes from browsers while letting other clients through. `raptor new <module> --spa` generates a project that also serves a single-page frontend from `public/` on the same origin.

A frontend that must live on another origin needs the `cors` middleware (`github.com/go-raptor/middlewares/cors`) registered in `config/components/middlewares.go`, its origin in `app.cors_allow_origins`, and the same origin in `app.csrf_trusted_origins`.
{{- end}}
````

The file must end with a newline after `{{- end}}`.

- [ ] **Step 8: Add the flag.** In `internal/new/new.go`, below `var Cmd = ...`, add:

```go
var spa bool

func init() {
	Cmd.Flags().BoolVar(&spa, "spa", false, "serve a single-page frontend from public/ with controllers/spa")
}
```

In `newProject`, change the copy call to `copyTemplateFiles(projectDir, moduleName, templateData{SPA: spa})`. After the final `fmt.Println("  raptor dev")`, add:

```go
	if spa {
		fmt.Println("\nFrontend: proxy /api to http://localhost:3000 in vite.config.ts, and copy its build into public/ for production (see README.md).")
	}
```

- [ ] **Step 9: Run the tests.**

Run: `go test -count=1 ./internal/new/ && go vet ./internal/new/ && gofmt -l internal/new`
Expected: `ok`, with no vet output and no gofmt output. Task 1's render test now also proves the conditional import block in the `--spa` `controllers.go` comes out gofmt-clean.

- [ ] **Step 10: Commit**

```bash
git add internal/new
git commit -F - <<'EOF'
new: add --spa, serving a frontend from public/ with controllers/spa

raptor new --spa registers &spa.SPAController{}, routes / to SPA.Index
with /api/v1/{path...} kept on the JSON 404, sets spa_optional in the
dev and test configs, ignores /public/, and documents the Vite proxy and
production. The default stays API-only.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
```

---

### Task 4: Generated projects vet and pass their own tests

**Files:**
- Modify: `internal/new/new.go`, extracting `createProject` from `newProject`.
- Test: `internal/new/new_test.go`

**Interfaces:**
- Consumes: `templateData`, `copyTemplateFiles` (Task 1), and `spa` (Task 3).
- Produces: `func createProject(projectDir, moduleName string, data templateData) error`, which creates the directory, the template, `go.mod` and runs `go mod tidy`.

- [ ] **Step 1: Write the build test.** Add `"os/exec"` to the imports of `internal/new/new_test.go`, and append:

```go
// Each variant, generated for real, vets and passes its own tests. For --spa that boots the SPA
// controller with no public/, through spa_optional in .raptor.test.yaml, and keeps the template's
// JSON 404 for an unknown API path. It needs the module proxy, so it skips under -short or offline.
func TestGeneratedProjectsPassTheirTests(t *testing.T) {
	if testing.Short() {
		t.Skip("generates, downloads and tests whole projects")
	}
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "APP_") { // a developer's app config must not leak into the generated tests
			env = append(env, kv)
		}
	}
	env = append(env, "GOWORK=off")
	// Probe with a module the template doesn't author, so a failure below is the template's own.
	probe := exec.Command("go", "list", "-m", "github.com/go-raptor/middlewares/csrf@latest")
	probe.Dir, probe.Env = t.TempDir(), env
	if out, err := probe.CombinedOutput(); err != nil {
		t.Skipf("the module proxy is unreachable (offline?): %v\n%s", err, out)
	}
	for variant, data := range map[string]templateData{"api": {}, "spa": {SPA: true}} {
		t.Run(variant, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "shop")
			if err := createProject(dir, "example.com/shop", data); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"vet", "./..."}, {"test", "-count=1", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir, cmd.Env = dir, env
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run it.**

Run: `go test -count=1 -run TestGeneratedProjectsPassTheirTests ./internal/new/`
Expected: a build failure, `undefined: createProject`.

- [ ] **Step 3: Extract `createProject`.** In `internal/new/new.go`, replace the body of `newProject` from `if err := os.MkdirAll(projectDir, 0755)` to just before `fmt.Printf("\n✅ Project %s created successfully!\n\n", projectDir)` with:

```go
	if err := createProject(projectDir, moduleName, templateData{SPA: spa}); err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(1)
	}
```

Then add below `newProject`:

```go
// createProject writes the template into projectDir with a go.mod, and resolves its
// dependencies.
func createProject(projectDir, moduleName string, data templateData) error {
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}
	if err := copyTemplateFiles(projectDir, moduleName, data); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}
	if err := writeGoMod(projectDir, moduleName); err != nil {
		return fmt.Errorf("creating go.mod: %w", err)
	}
	fmt.Println("Resolving dependencies... 📦")
	if err := runInDir(projectDir, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run it.**

Run: `go test -count=1 -run TestGeneratedProjectsPassTheirTests -v ./internal/new/`
Expected: `--- PASS: TestGeneratedProjectsPassTheirTests/api` and `--- PASS: TestGeneratedProjectsPassTheirTests/spa`. If the machine is offline you'll see `SKIP` with the probe's error instead, and this task can't be completed offline.

- [ ] **Step 5: Prove the build test catches a broken `--spa` project.** The test covers behaviour the earlier tasks already built, so show that it has teeth:
  1. Temporarily delete the three lines `app:`, the `# Boot without…` comment, and `spa_optional: "true"` from `internal/new/_template/.raptor.test.yaml.tmpl`.
  2. Run: `go test -count=1 -run TestGeneratedProjectsPassTheirTests ./internal/new/`. Expected: `--- FAIL: TestGeneratedProjectsPassTheirTests/spa`, whose output contains `spa: reading directory` and `public`.
  3. Restore the file: `git checkout internal/new/_template/.raptor.test.yaml.tmpl`.
  4. Run the full suite: `go test -count=1 ./internal/new/`. Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/new
git commit -F - <<'EOF'
new: test that generated projects vet and pass their own tests

createProject holds what newProject did between the directory check and
the success message, so a test can generate both variants for real and
run go vet and go test inside them. The --spa run proves the project
boots with no frontend build. The test skips under -short or offline.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
```

---

### Task 5: Documentation and skills

**Files:**
- Modify: `README.md` (CLI), adding a new *Creating a project* section after *Installation*.
- Modify: `CHANGELOG.md` (CLI), adding a new `## Unreleased` section at the top.
- Modify: `~/.claude/skills/raptor-api-conventions/SKILL.md`, in the scaffolding list under "Scaffold with the CLI when it is installed:".
- Modify: `~/.claude/skills/sveltekit-conventions/references/setup.md`, in *Development* and *Deployment*.

**Interfaces:** none. This task is prose only, so it needs no tests.

- [ ] **Step 1: CLI README.** Insert this before `## Generated configuration`:

````markdown
## Creating a project

```bash
raptor new github.com/you/shop
```

creates an API-only project in `shop/`. It is same-origin: there is no CORS, and the csrf middleware (`github.com/go-raptor/middlewares/csrf`) rejects cross-origin writes from browsers while letting other clients through.

`--spa` also serves a single-page frontend (a SvelteKit `adapter-static` build, or any equivalent) from `public/` with `controllers/spa`, on the same origin:

- `SPA.Index` is routed on `/`, and `/api/v1/{path...}` keeps Raptor's JSON 404.
- `.raptor.dev.yaml` and `.raptor.test.yaml` set `spa_optional: "true"`, so the project boots and its tests pass before a frontend is built. Production must not load it.
- The generated README shows the Vite proxy for development: `http://localhost:3000` in its object form, so csrf sees the original `Host`.

A frontend that must live on another origin needs the `cors` middleware registered, its origin in `app.cors_allow_origins`, and the same origin in `app.csrf_trusted_origins`.

````

- [ ] **Step 2: CLI CHANGELOG.** Insert this directly under `# Changelog`:

```markdown
## Unreleased

### Changed

- **Behavior:** `raptor new` no longer registers the `cors` middleware or sets `cors_allow_origins: "http://localhost:5173"`, a cross-origin development setup. New projects are same-origin and register the `csrf` middleware instead. Proxy the frontend's dev server to the API (see the README), or add `cors` back for a frontend that must live on another origin.
- Generated Go files are gofmt-clean: imports are sorted after the module path is filled in.

### Added

- `raptor new --spa` serves a single-page frontend from `public/` with `controllers/spa`. It adds the `SPA.Index` catch-all route, `spa_optional` in `.raptor.dev.yaml` and `.raptor.test.yaml`, `/public/` in `.gitignore`, and README instructions for the Vite proxy and production.
```

- [ ] **Step 3: Commit the CLI docs.**

```bash
git add README.md CHANGELOG.md
git commit -F - <<'EOF'
docs: document raptor new's same-origin default and --spa

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
```

- [ ] **Step 4: raptor-api-conventions.** In `~/.claude/skills/raptor-api-conventions/SKILL.md`, insert this as the first bullet under `Scaffold with the CLI when it is installed:`:

```markdown
- `raptor new <module> --spa` (CLI v1.4.0+) starts an app the way these conventions run it: same-origin, csrf, and spa/v2 serving `public/` with `spa_optional` in the dev and test configs. Without `--spa` it is API-only, still with csrf and no CORS.
```

- [ ] **Step 5: sveltekit-conventions.** In `~/.claude/skills/sveltekit-conventions/references/setup.md`:
  - Append this paragraph after the one that starts ``Open `http://localhost:5173`.``:

    ```markdown
    A backend created with `raptor new <module> --spa` (CLI v1.4.0+) is already wired this way: spa/v2 on `/`, csrf, and `spa_optional` in its dev and test configs.
    ```

  - Replace ``The Go binary serves the built SPA with the spa/v2 controller (`Directory: "public"`), so one image holds both halves:`` with ``The Go binary serves the built SPA from `public/` with the spa/v2 controller, so one image holds both halves:``.
  - Leave the proxy target in this file's `vite.config.ts` example (`http://127.0.0.1:3000`) alone. The spec doesn't change it.

- [ ] **Step 6: Commit the skills.** Stage only the two skill files. `synced/` is not ours.

```bash
cd ~/.claude/skills
git add raptor-api-conventions/SKILL.md sveltekit-conventions/references/setup.md
git commit -F - <<'EOF'
Point new apps at raptor new --spa

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_017SB9ZHXjZp3hKprX5mxB35
EOF
cd -
```

---

### Task 6: Release CLI v1.4.0 (only after an explicit go-ahead)

**Files:**
- Modify: `internal/version/version.go`, changing `version = "v1.3.1"` to `"v1.4.0"`.
- Modify: `CHANGELOG.md`, changing `## Unreleased` to `## v1.4.0 — <release date>`.

- [ ] **Step 1: Ask for the go-ahead** to merge, push and tag, and wait for it.
- [ ] **Step 2: Bump and commit.** Change both files, then run `git commit -am` with the subject `Bump version to v1.4.0` and the two trailers from Global Constraints.
- [ ] **Step 3: Merge.** `git switch main && git merge --ff-only raptor-new-spa`, then run `go test -count=1 ./... && go vet ./...` on the merged result. Expected: every package `ok`. Then run `go run ./cmd/raptor version`. Expected: `Raptor CLI v1.4.0`. Finally `git branch -d raptor-new-spa`.
- [ ] **Step 4: Push and tag.**
  - `git push origin main && git push gh main`
  - `git tag v1.4.0 && git push gh v1.4.0`, so the tag goes to gh only.
  - Confirm with `git ls-remote` that both remotes' `main` and gh's tag point at HEAD, and that origin has no `v1.4.0`.
- [ ] **Step 5: Push the skills.** `git -C ~/.claude/skills push origin main`.
