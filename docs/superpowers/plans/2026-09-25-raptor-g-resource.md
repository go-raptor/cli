# `raptor g resource` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `raptor g resource <Name> [field:type ...] [--parent X] [--movable] [--plural Y]`. It scaffolds a complete entity (model, DTOs, zog schema, service with ownership checks, controller, routes, migration and integration tests) exactly as the raptor-api-conventions skill writes one.

**Architecture:** A new package `internal/generate/resource` does the work in stages:
1. It parses the command line into a `Resource`.
2. It reads the target project: the `go.mod`, a Bun model index built with `go/parser`, declarations in the services, controllers and tests, the routes and the migrations.
3. It derives the ownership chain from the model index.
4. It builds one `view` struct and renders embedded `text/template` files through `go/format`.
5. It writes every new file only after all checks pass, then applies anchored edits to existing files. Each edit parses its result before writing, or prints the change for the user to make by hand.

Two small packages come out of `generate.go`: `internal/naming` (case conversions) and `internal/components` (component registration).

**Tech Stack:** Go 1.27 toolchain (cli `go.mod` stays `go 1.26`), `go/parser`, `go/ast`, `go/format`, `text/template`, `embed`, cobra, and `go.yaml.in/yaml/v3` (new). Generated code targets Raptor v4.4+, Bun 1.2, zog 0.23, pgx 5 and `connectors/bun/postgres` v1.2+.

**Spec:** `docs/superpowers/specs/2026-09-25-raptor-g-resource-design.md`, approved 2026-09-25. The conventions it emits are defined in `~/.claude/skills/raptor-api-conventions` at `h00s/claude-skills@f17639e`.

## Global Constraints

- The cli module is `github.com/go-raptor/cli`. Keep the `go 1.26` directive. The generated project needs Go 1.27, because Raptor v4.4 does.
- The existing `controller`, `service`, `middleware` and `model` generators keep their exact behavior and output.
- The generated code matches `patterns.md`, `database.md` and `testing.md` at `claude-skills@f17639e`.
- Never overwrite a file, and there is no `--force`. When any target exists, nothing is written.
- An edit to an existing file is either applied cleanly (gofmt'd Go, parsed YAML) or printed as a manual step. A failed edit never leaves a corrupted file.
- The only new dependency is `go.yaml.in/yaml/v3 v3.0.5`.
- Make one commit per task in `~/dev/go/go-raptor/cli` on `main`. End every commit message with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **No tag and no push:** releasing v1.3.0 needs the user's go-ahead.
- Every commit leaves `go test ./...`, `go vet ./...` and `gofmt -l .` clean. `go test -short ./...` must also pass without network access.

## Review Focus

1. **Identifiers that collide with Go or the generated code.** A resource named `Type`, `Err` or `List`, or one whose model already exists (`User`), must get a clear error, never uncompilable output. Tested in Tasks 3 and 11.
2. **A hand-written parent that also references shared data.** A parent model with an FK to reference data besides its owner FK, like a `Lesson` with `OutcomeID` and `DivisionID`, must yield a chain through the owned FK only. Tested in Task 5.
3. **A `routes.yaml` shaped differently from the template:** 4-space indentation, `/api/v1:` as the last block, a missing trailing newline, CRLF line endings, or no `/api/v1:` at all. The edit must adapt or print; it must never corrupt the file. Tested in Task 12.
4. **Multiword names** (`LectureGroup`, `lecture_hours`, `url_path`). Snake, camel, kebab, plural and initialisms must agree across all files: table `lecture_groups`, route `/lecture-groups`, JSON `lectureGroupId`, Go `LectureGroupID`. Tested in Tasks 2 and 6.
5. **Running the generator twice, or over an existing target file.** It must refuse before writing anything. Tested in Task 13.

## File Structure

| Path | Responsibility |
|---|---|
| `internal/naming/naming.go` | `Snake`, `Pascal` (moved from `generate.go`), `GoField`, `Camel`, `Var`, `Kebab`, `Plural`, `Label`, `LabelLower`, `Reserved` |
| `internal/components/components.go` | `AddEntry` (pure edit of a registration file) and `Register` (read, edit, write), moved from `generate.go` |
| `internal/generate/generate.go` | Adds the `resource` type, its flags and `checkArgs` |
| `internal/generate/resource/spec.go` | `Field`, `Resource`, `ParseField`, `NewResource` |
| `internal/generate/resource/models.go` | `ModelIndex`: Bun structs parsed from `app/models`; `FKTarget` |
| `internal/generate/resource/ownership.go` | `IsOwned`, `OwnerChain`, `Chain.Predicate` |
| `internal/generate/resource/view.go` | `view` and `buildView`: every name, type, tag, schema, SQL fragment and sample the templates need |
| `internal/generate/resource/render.go` | Embedded templates, `renderGo`, `renderMigration`, `setupMigration` |
| `internal/generate/resource/seed.go` | `buildSeedModel`: seed functions for models the generator didn't write |
| `internal/generate/resource/project.go` | `Project`, `Inspect`, `decls`, `decide` (preconditions and bootstrap decisions) |
| `internal/generate/resource/edits.go` | `addValidationSchema`, `addSchemaTest`, `addRoutes` |
| `internal/generate/resource/run.go` | `Plan`, `Run`, `Options`, `Generation`, `File`, `Edit` |
| `internal/generate/resource/templates/*.tmpl` | model, service, controller, database_service, validation_service, helpers, validation, schemas_test, setup_test, harness_test, seed, seed_model, controller_test |
| `internal/generate/resource/testdata/models/*.go` | Model sources for the index, ownership and view tests |
| `internal/generate/resource/testdata/project/` | A minimal Raptor project with the auth stack, for the `Inspect`, `Run`, golden and compile tests |
| `internal/generate/resource/testdata/golden/` | Frozen output for the `course`, `outcome`, `unit` (movable, depth 2) and `record` (depth 3) scenarios |

---

### Task 1: Extract `naming` and `components` from `generate.go`

A pure refactor. `generate.go` has no tests today, so this task pins the moved behavior with characterization tests first.

**Files:**
- Create: `internal/naming/naming.go`, `internal/naming/naming_test.go`, `internal/components/components.go`, `internal/components/components_test.go`
- Modify: `internal/generate/generate.go` (delete `toSnakeCase`, `toPascalCase` and `registerComponent`; call the new packages)

**Interfaces:**
- Produces: `naming.Snake(s string) string` and `naming.Pascal(s string) string`, the existing behavior.
- Produces: `components.AddEntry(src, moduleName, kind, structName string) (string, error)`, where `kind` is `"controller"` or `"service"`.
- Produces: `components.Register(moduleName, kind, structName string) error`, which reads and writes `config/components/<kind>s.go`.

- [ ] **Step 1: Write the failing tests.**

`internal/naming/naming_test.go`:

```go
package naming

import "testing"

func TestSnake(t *testing.T) {
	for in, want := range map[string]string{
		"Users":         "users",
		"RateLimit":     "rate_limit",
		"HTTPServer":    "http_server",
		"APIKey":        "api_key",
		"LectureGroup":  "lecture_group",
		"already_snake": "already_snake",
	} {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPascal(t *testing.T) {
	for in, want := range map[string]string{
		"rate_limit": "RateLimit",
		"user_api":   "UserApi", // no initialisms: existing generators name structs this way
		"users":      "Users",
	} {
		if got := Pascal(in); got != want {
			t.Errorf("Pascal(%q) = %q, want %q", in, got, want)
		}
	}
}
```

`internal/components/components_test.go`:

```go
package components

import (
	"strings"
	"testing"
)

const servicesSrc = `package components

import (
	"github.com/go-raptor/raptor/v4"
	"example.com/shop/app/services"
)

func Services() raptor.Services {
	return raptor.Services{
		&services.HelloService{},
	}
}
`

func TestAddEntryAppendsToTheSlice(t *testing.T) {
	out, err := AddEntry(servicesSrc, "example.com/shop", "service", "CoursesService")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\t\t&services.HelloService{},\n\t\t&services.CoursesService{},\n\t}"; !strings.Contains(out, want) {
		t.Fatalf("entry not appended:\n%s", out)
	}
}

func TestAddEntryAddsTheImport(t *testing.T) {
	src := strings.Replace(servicesSrc, "\t\"example.com/shop/app/services\"\n", "", 1)
	out, err := AddEntry(src, "example.com/shop", "service", "CoursesService")
	if err != nil {
		t.Fatal(err)
	}
	if want := "\"github.com/go-raptor/raptor/v4\"\n\t\"example.com/shop/app/services\""; !strings.Contains(out, want) {
		t.Fatalf("import not added:\n%s", out)
	}
}

func TestAddEntryIsIdempotent(t *testing.T) {
	out, err := AddEntry(servicesSrc, "example.com/shop", "service", "HelloService")
	if err != nil || out != servicesSrc {
		t.Fatalf("an already registered component must leave the file unchanged: %v\n%s", err, out)
	}
}

func TestAddEntryWithoutTheSliceFails(t *testing.T) {
	src := "package components\n\nimport (\n\t\"github.com/go-raptor/raptor/v4\"\n)\n"
	if _, err := AddEntry(src, "example.com/shop", "service", "CoursesService"); err == nil {
		t.Fatal("a file without raptor.Services{ must be reported, not guessed at")
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/naming/ ./internal/components/`. Expected: FAIL (`no non-test Go files` / undefined `Snake`, `AddEntry`).

- [ ] **Step 3: Implement.** In `internal/naming/naming.go`, `Snake` and `Pascal` are the bodies of `toSnakeCase` and `toPascalCase` from `generate.go`, moved verbatim:

```go
// Package naming converts identifiers between the case styles the generators emit.
package naming

import (
	"strings"
	"unicode"
)

// Snake converts PascalCase or camelCase to snake_case, keeping runs of capitals together:
// "HTTPServer" → "http_server", "LectureGroup" → "lecture_group".
func Snake(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result = append(result, '_')
				} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
					result = append(result, '_')
				}
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Pascal converts snake_case to PascalCase without initialisms: "user_api" → "UserApi".
func Pascal(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
```

`internal/components/components.go`. `AddEntry` is `registerComponent`'s body without the file I/O:

```go
// Package components registers controllers and services in config/components.
package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddEntry adds &<pkg>.<structName>{} to the Controllers or Services slice in src, importing
// the package if needed. An already registered component leaves src unchanged.
func AddEntry(src, moduleName, kind, structName string) (string, error) {
	var importPkg, marker, entry string
	switch kind {
	case "controller":
		importPkg = moduleName + "/app/controllers"
		marker = "raptor.Controllers{"
		entry = fmt.Sprintf("&controllers.%s{},", structName)
	case "service":
		importPkg = moduleName + "/app/services"
		marker = "raptor.Services{"
		entry = fmt.Sprintf("&services.%s{},", structName)
	default:
		return "", fmt.Errorf("unsupported component type for registration: %s", kind)
	}

	s := src
	if strings.Contains(s, structName) {
		return s, nil
	}

	if !strings.Contains(s, importPkg) {
		raptorImport := "\"github.com/go-raptor/raptor/v4\""
		idx := strings.Index(s, raptorImport)
		if idx == -1 {
			return "", fmt.Errorf("could not find the raptor import")
		}
		lineEnd := idx + len(raptorImport)
		s = s[:lineEnd] + fmt.Sprintf("\n\t\"%s\"", importPkg) + s[lineEnd:]
	}

	markerIdx := strings.Index(s, marker)
	if markerIdx == -1 {
		return "", fmt.Errorf("could not find %s", marker)
	}
	depth := 0
	closingIdx := -1
	for i := markerIdx + strings.Index(marker, "{"); i < len(s); i++ {
		if s[i] == '{' {
			depth++
		}
		if s[i] == '}' {
			depth--
			if depth == 0 {
				closingIdx = i
				break
			}
		}
	}
	if closingIdx == -1 {
		return "", fmt.Errorf("could not find the closing brace of %s", marker)
	}
	nlIdx := strings.LastIndex(s[:closingIdx], "\n")
	return s[:nlIdx+1] + fmt.Sprintf("\t\t%s\n", entry) + s[nlIdx+1:], nil
}

// Register adds structName to config/components/<kind>s.go.
func Register(moduleName, kind, structName string) error {
	regFile := filepath.Join("config", "components", kind+"s.go")
	content, err := os.ReadFile(regFile)
	if err != nil {
		return err
	}
	out, err := AddEntry(string(content), moduleName, kind, structName)
	if err != nil {
		return fmt.Errorf("%s: %w", regFile, err)
	}
	if out == string(content) {
		return nil
	}
	return os.WriteFile(regFile, []byte(out), 0644)
}
```

In `generate.go`:
- Delete `toSnakeCase`, `toPascalCase` and `registerComponent`, and the now-unused `unicode` import.
- Replace `toSnakeCase(name)` with `naming.Snake(name)` and `toPascalCase(snakeName)` with `naming.Pascal(snakeName)`.
- Replace both `registerComponent(moduleName, "controller"|"service", structName)` calls with `components.Register(...)`.
- Add the imports `github.com/go-raptor/cli/internal/components` and `github.com/go-raptor/cli/internal/naming`.

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS, and gofmt prints nothing.
- [ ] **Step 5:** Smoke-test that the old generators are unchanged:

```bash
S=$(mktemp -d) && go build -o $S/raptor ./cmd/raptor && cp -r internal/new/_template $S/app && cd $S/app && printf 'module example.com/app\n\ngo 1.27\n' > go.mod && sed -i 's#github.com/go-raptor/template#example.com/app#' config/components/*.go app/controllers/*_test.go app/services/*_test.go && $S/raptor g controller Things && grep -q 'controllers.ThingsController{}' config/components/controllers.go && echo OK; cd - >/dev/null
```

Expected: `Created app/controllers/things_controller.go`, `Registered ThingsController…`, then `OK`.

- [ ] **Step 6:** Commit.

```bash
git add internal/naming internal/components internal/generate/generate.go
git commit -m "Extract naming and component registration from the generator

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Naming conversions for generated code

**Files:** Modify `internal/naming/naming.go` and `internal/naming/naming_test.go`.

**Interfaces:**
- Consumes: `Snake` (Task 1).
- Produces:

```go
func GoField(snake string) string    // "division_id" → "DivisionID"
func Camel(snake string) string      // "division_id" → "divisionId"
func Var(pascal string) string       // "LectureGroups" → "lectureGroups"
func Kebab(snake string) string      // "lecture_groups" → "lecture-groups"
func Plural(pascal string) string    // "Category" → "Categories"
func Label(pascal string) string     // "LectureGroup" → "Lecture group"
func LabelLower(pascal string) string // "LectureGroup" → "lecture group"
func Reserved(ident string) bool     // Go keywords and predeclared identifiers
```

- [ ] **Step 1: Write the failing tests** (append to `naming_test.go`):

```go
func TestGoField(t *testing.T) {
	for in, want := range map[string]string{
		"division_id":   "DivisionID",
		"url_path":      "URLPath",
		"lecture_hours": "LectureHours",
		"starts_at":     "StartsAt",
		"api_key":       "APIKey",
	} {
		if got := GoField(in); got != want {
			t.Errorf("GoField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamel(t *testing.T) {
	for in, want := range map[string]string{
		"division_id":      "divisionId",
		"lecture_group_id": "lectureGroupId",
		"name":             "name",
		"url_path":         "urlPath",
	} {
		if got := Camel(in); got != want {
			t.Errorf("Camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVarKebabPlural(t *testing.T) {
	for in, want := range map[string]string{"Course": "course", "LectureGroups": "lectureGroups", "APIKey": "apiKey"} {
		if got := Var(in); got != want {
			t.Errorf("Var(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Kebab("lecture_groups"); got != "lecture-groups" {
		t.Errorf("Kebab = %q", got)
	}
	for in, want := range map[string]string{
		"Course": "Courses", "Status": "Statuses", "Box": "Boxes", "Match": "Matches",
		"Category": "Categories", "Day": "Days", "LectureGroup": "LectureGroups",
	} {
		if got := Plural(in); got != want {
			t.Errorf("Plural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLabels(t *testing.T) {
	for in, want := range map[string][2]string{
		"Course":       {"Course", "course"},
		"LectureGroup": {"Lecture group", "lecture group"},
		"APIKey":       {"API key", "API key"},
	} {
		if got := Label(in); got != want[0] {
			t.Errorf("Label(%q) = %q, want %q", in, got, want[0])
		}
		if got := LabelLower(in); got != want[1] {
			t.Errorf("LabelLower(%q) = %q, want %q", in, got, want[1])
		}
	}
}

func TestReserved(t *testing.T) {
	for _, ident := range []string{"type", "range", "string", "len", "error", "new"} {
		if !Reserved(ident) {
			t.Errorf("Reserved(%q) = false", ident)
		}
	}
	if Reserved("course") {
		t.Error(`Reserved("course") = true`)
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/naming/`. Expected: FAIL (undefined `GoField` and the others).
- [ ] **Step 3: Implement** (append to `naming.go`):

```go
// initialisms are the words Go spells in capitals inside identifiers.
var initialisms = map[string]string{
	"id": "ID", "url": "URL", "api": "API", "ip": "IP", "uuid": "UUID",
	"html": "HTML", "http": "HTTP", "json": "JSON",
}

// GoField converts snake_case to an exported Go identifier with initialisms:
// "division_id" → "DivisionID", "url_path" → "URLPath".
func GoField(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		if up, ok := initialisms[part]; ok {
			b.WriteString(up)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// Camel converts snake_case to the lowerCamel JSON member style the conventions use, with no
// initialisms: "division_id" → "divisionId".
func Camel(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(part)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// Var is the local variable name for a PascalCase type name: "LectureGroups" → "lectureGroups".
func Var(pascal string) string { return Camel(Snake(pascal)) }

// Kebab converts snake_case to the kebab-case of a route segment.
func Kebab(snake string) string { return strings.ReplaceAll(snake, "_", "-") }

// Plural applies the regular English rules; irregular nouns take --plural instead.
func Plural(word string) string {
	lower := strings.ToLower(word)
	for _, suffix := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(lower, suffix) {
			return word + "es"
		}
	}
	if n := len(lower); n > 1 && lower[n-1] == 'y' && !strings.ContainsRune("aeiou", rune(lower[n-2])) {
		return word[:len(word)-1] + "ies"
	}
	return word + "s"
}

// Label is a type name as words for messages: "LectureGroup" → "Lecture group".
func Label(pascal string) string {
	words := strings.Split(Snake(pascal), "_")
	for i, w := range words {
		if up, ok := initialisms[w]; ok {
			words[i] = up
		} else if i == 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// LabelLower is Label mid-sentence: "LectureGroup" → "lecture group".
func LabelLower(pascal string) string {
	words := strings.Split(Snake(pascal), "_")
	for i, w := range words {
		if up, ok := initialisms[w]; ok {
			words[i] = up
		}
	}
	return strings.Join(words, " ")
}

var reserved = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true, "default": true,
	"defer": true, "else": true, "fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true, "map": true, "package": true,
	"range": true, "return": true, "select": true, "struct": true, "switch": true, "type": true,
	"var": true,
	"any": true, "bool": true, "byte": true, "comparable": true, "complex64": true,
	"complex128": true, "error": true, "float32": true, "float64": true, "int": true, "int8": true,
	"int16": true, "int32": true, "int64": true, "rune": true, "string": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true, "true": true,
	"false": true, "iota": true, "nil": true, "append": true, "cap": true, "clear": true,
	"close": true, "complex": true, "copy": true, "delete": true, "imag": true, "len": true,
	"make": true, "max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
}

// Reserved reports whether ident is a Go keyword or predeclared identifier, which a generated
// variable must not use.
func Reserved(ident string) bool { return reserved[ident] }
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `Add naming conversions for generated code`, plus the trailer.

---

### Task 3: Parse the resource name and field specs

**Files:** Create `internal/generate/resource/spec.go` and `internal/generate/resource/spec_test.go`.

**Interfaces:**
- Consumes: `naming.GoField`, `naming.Camel`, `naming.Plural`, `naming.Var`, `naming.Snake`, `naming.Reserved` (Task 2).
- Produces:

```go
type FieldType int // String, Text, Int, Int64, Bool, Time, Enum, Ref

type Field struct {
	Name       string   // snake_case as given; a ref's name has no _id
	Type       FieldType
	Length     int      // String: VARCHAR length, 150 by default
	EnumValues []string // Enum
	RefModel   string   // Ref: target model, PascalCase
	Optional   bool
}
func (f Field) Column() string // a ref's name + "_id"
func (f Field) GoName() string // naming.GoField(Column())
func (f Field) JSON() string   // naming.Camel(Column())

type Resource struct {
	Name, Plural string // PascalCase
	Fields       []Field
	Parent       string // PascalCase model, "" when flat
	Movable      bool
}
func ParseField(spec string) (Field, error)
func NewResource(name string, specs []string, parent, plural string, movable bool) (*Resource, error)
```

- [ ] **Step 1: Write the failing tests** (`spec_test.go`):

```go
package resource

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseField(t *testing.T) {
	tests := []struct {
		spec string
		want Field
	}{
		{"name:string", Field{Name: "name", Type: String, Length: 150}},
		{"title:string:80", Field{Name: "title", Type: String, Length: 80}},
		{"nickname:string:optional", Field{Name: "nickname", Type: String, Length: 150, Optional: true}},
		{"code:string:12:optional", Field{Name: "code", Type: String, Length: 12, Optional: true}},
		{"notes:text", Field{Name: "notes", Type: Text}},
		{"lecture_hours:int", Field{Name: "lecture_hours", Type: Int}},
		{"size:int64:optional", Field{Name: "size", Type: Int64, Optional: true}},
		{"active:bool", Field{Name: "active", Type: Bool}},
		{"starts_at:time:optional", Field{Name: "starts_at", Type: Time, Optional: true}},
		{"type:enum:lecture,lab", Field{Name: "type", Type: Enum, EnumValues: []string{"lecture", "lab"}}},
		{"division:ref", Field{Name: "division", Type: Ref, RefModel: "Division"}},
		{"mentor:ref:User:optional", Field{Name: "mentor", Type: Ref, RefModel: "User", Optional: true}},
		{"api_key:ref", Field{Name: "api_key", Type: Ref, RefModel: "APIKey"}},
	}
	for _, tt := range tests {
		got, err := ParseField(tt.spec)
		if err != nil {
			t.Errorf("ParseField(%q): %v", tt.spec, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseField(%q) = %+v, want %+v", tt.spec, got, tt.want)
		}
	}
}

func TestFieldNames(t *testing.T) {
	f := Field{Name: "division", Type: Ref, RefModel: "Division"}
	if f.Column() != "division_id" || f.GoName() != "DivisionID" || f.JSON() != "divisionId" {
		t.Fatalf("ref names: %q %q %q", f.Column(), f.GoName(), f.JSON())
	}
	f = Field{Name: "url_path", Type: String}
	if f.Column() != "url_path" || f.GoName() != "URLPath" || f.JSON() != "urlPath" {
		t.Fatalf("multiword names: %q %q %q", f.Column(), f.GoName(), f.JSON())
	}
}

func TestParseFieldErrors(t *testing.T) {
	for spec, want := range map[string]string{
		"name":                  "expected name:type",
		"Name:string":           "snake_case",
		"name:varchar":          `unknown type "varchar"`,
		"name:string:0":         "string length",
		"notes:text:optional":   "text is already optional",
		"notes:text:5":          `unexpected "5"`,
		"type:enum":             "enum needs its values",
		"type:enum:Lecture":     `enum value "Lecture"`,
		"type:enum:a,a":         `duplicate enum value "a"`,
		"division:ref:division": "PascalCase model name",
		"order:string":          `"order" is reserved`,
		"user:ref":              `"user" is reserved`,
		"created_at:time":       `"created_at" is reserved`,
		"course_id:int64":       "course:ref",
	} {
		_, err := ParseField(spec)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ParseField(%q) error = %v, want it to mention %q", spec, err, want)
		}
	}
}

func TestNewResource(t *testing.T) {
	r, err := NewResource("LectureGroup", nil, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Plural != "LectureGroups" || len(r.Fields) != 1 || !reflect.DeepEqual(r.Fields[0], Field{Name: "name", Type: String, Length: 150}) {
		t.Fatalf("defaults: %+v", r)
	}
	r, err = NewResource("Criterion", []string{"name:string"}, "", "Criteria", false)
	if err != nil || r.Plural != "Criteria" {
		t.Fatalf("plural override: %+v %v", r, err)
	}
}

func TestNewResourceErrors(t *testing.T) {
	tests := []struct {
		name    string
		specs   []string
		parent  string
		plural  string
		movable bool
		want    string
	}{
		{"course", nil, "", "", false, "PascalCase"},
		{"Type", nil, "", "", false, "Go keyword"},
		{"Err", nil, "", "", false, "used by the generated code"},
		{"List", nil, "", "", false, "used by the generated code"},
		{"Course", nil, "", "", true, "--movable only applies with --parent"},
		{"Course", nil, "Course", "", false, "its own parent"},
		{"Outcome", []string{"course:ref"}, "Course", "", false, "added by --parent"},
		{"Course", []string{"name:string", "name:text"}, "", "", false, "declared twice"},
		{"Course", []string{"division:ref", "division:string"}, "", "", false, "Division is used twice"},
		{"Course", nil, "", "courses", false, "--plural"},
	}
	for _, tt := range tests {
		_, err := NewResource(tt.name, tt.specs, tt.parent, tt.plural, tt.movable)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("NewResource(%q, %v, parent %q) error = %v, want it to mention %q", tt.name, tt.specs, tt.parent, err, tt.want)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/`. Expected: FAIL (undefined `ParseField`).
- [ ] **Step 3: Implement** `spec.go`:

```go
// Package resource implements `raptor g resource`: one command that scaffolds a whole entity
// the way the raptor-api-conventions skill writes one.
package resource

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

type FieldType int

const (
	String FieldType = iota
	Text
	Int
	Int64
	Bool
	Time
	Enum
	Ref
)

var fieldTypes = map[string]FieldType{
	"string": String, "text": Text, "int": Int, "int64": Int64,
	"bool": Bool, "time": Time, "enum": Enum, "ref": Ref,
}

const defaultStringLength = 150

// Field is one field spec from the command line: name:type[:arg][:optional].
type Field struct {
	Name       string   // snake_case as given; a ref's name has no _id
	Type       FieldType
	Length     int      // String: VARCHAR length
	EnumValues []string // Enum
	RefModel   string   // Ref: target model, PascalCase
	Optional   bool
}

// Column is the database column; a ref's name gains _id.
func (f Field) Column() string {
	if f.Type == Ref {
		return f.Name + "_id"
	}
	return f.Name
}

func (f Field) GoName() string { return naming.GoField(f.Column()) }
func (f Field) JSON() string   { return naming.Camel(f.Column()) }

// Resource is everything the command line says about the entity to generate.
type Resource struct {
	Name    string // PascalCase singular: LectureGroup
	Plural  string // PascalCase plural: LectureGroups
	Fields  []Field
	Parent  string // PascalCase parent model; "" for a flat resource
	Movable bool
}

var (
	pascalRe = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	snakeRe  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	enumRe   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// reservedColumns are columns every generated table already has, plus words PostgreSQL reserves.
var reservedColumns = map[string]bool{
	"id": true, "user_id": true, "created_at": true, "updated_at": true,
	"user": true, "order": true, "group": true, "table": true, "select": true, "default": true,
	"check": true, "references": true, "limit": true, "offset": true, "desc": true, "asc": true,
	"primary": true, "foreign": true, "unique": true, "end": true, "from": true, "where": true, "to": true,
}

// generatedIdents are local names the generated code already uses, so a resource's variable
// name must not take one.
var generatedIdents = map[string]bool{
	"s": true, "c": true, "r": true, "m": true, "t": true, "i": true, "x": true, "err": true,
	"res": true, "exists": true, "id": true, "ctx": true, "user": true, "req": true, "rec": true,
	"raw": true, "path": true, "list": true, "session": true, "owner": true, "stranger": true,
	"strangerSession": true, "suffix": true, "created": true, "updated": true, "listed": true,
	"body": true, "reader": true, "opts": true, "encoded": true,
}

// ParseField parses one name:type[:arg][:optional] spec.
func ParseField(spec string) (Field, error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		return Field{}, fmt.Errorf("field %q: expected name:type", spec)
	}
	f := Field{Name: parts[0]}
	if !snakeRe.MatchString(f.Name) {
		return Field{}, fmt.Errorf("field %q: the name must be snake_case, starting with a letter", spec)
	}
	typ, ok := fieldTypes[parts[1]]
	if !ok {
		return Field{}, fmt.Errorf("field %q: unknown type %q (use string, text, int, int64, bool, time, enum or ref)", spec, parts[1])
	}
	f.Type = typ
	args := parts[2:]
	if n := len(args); n > 0 && args[n-1] == "optional" {
		f.Optional = true
		args = args[:n-1]
	}
	switch typ {
	case String:
		f.Length = defaultStringLength
		if len(args) == 1 {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 1 || n > 10485760 {
				return Field{}, fmt.Errorf("field %q: the string length must be a number from 1 to 10485760", spec)
			}
			f.Length, args = n, nil
		}
	case Text:
		if f.Optional {
			return Field{}, fmt.Errorf("field %q: text is already optional", spec)
		}
	case Enum:
		if len(args) != 1 || args[0] == "" {
			return Field{}, fmt.Errorf("field %q: enum needs its values, e.g. %s:enum:draft,published", spec, f.Name)
		}
		seen := map[string]bool{}
		for _, v := range strings.Split(args[0], ",") {
			if !enumRe.MatchString(v) {
				return Field{}, fmt.Errorf("field %q: enum value %q must be lowercase letters, digits and underscores", spec, v)
			}
			if seen[v] {
				return Field{}, fmt.Errorf("field %q: duplicate enum value %q", spec, v)
			}
			seen[v] = true
			f.EnumValues = append(f.EnumValues, v)
		}
		args = nil
	case Ref:
		f.RefModel = naming.GoField(f.Name)
		if len(args) == 1 {
			if !pascalRe.MatchString(args[0]) {
				return Field{}, fmt.Errorf("field %q: the ref target must be a PascalCase model name", spec)
			}
			f.RefModel, args = args[0], nil
		}
	}
	if len(args) > 0 {
		return Field{}, fmt.Errorf("field %q: unexpected %q", spec, strings.Join(args, ":"))
	}
	if reservedColumns[f.Name] || reservedColumns[f.Column()] {
		return Field{}, fmt.Errorf("field %q: %q is reserved", spec, f.Name)
	}
	if f.Type != Ref && strings.HasSuffix(f.Name, "_id") {
		return Field{}, fmt.Errorf("field %q: a foreign key is %s:ref, not %s", spec, strings.TrimSuffix(f.Name, "_id"), f.Name)
	}
	return f, nil
}

// NewResource validates the name and flags and parses the field specs. With no specs, the
// resource gets a single name:string.
func NewResource(name string, specs []string, parent, plural string, movable bool) (*Resource, error) {
	if !pascalRe.MatchString(name) {
		return nil, fmt.Errorf("resource name %q must be PascalCase, e.g. Course or LectureGroup", name)
	}
	r := &Resource{Name: name, Plural: naming.Plural(name), Parent: parent, Movable: movable}
	if plural != "" {
		if !pascalRe.MatchString(plural) {
			return nil, fmt.Errorf("--plural %q must be PascalCase, e.g. Criteria", plural)
		}
		r.Plural = plural
	}
	for _, ident := range []string{naming.Var(r.Name), naming.Var(r.Plural)} {
		if naming.Reserved(ident) {
			return nil, fmt.Errorf("resource name %q: %q is a Go keyword or predeclared identifier", name, ident)
		}
		if generatedIdents[ident] {
			return nil, fmt.Errorf("resource name %q: %q is used by the generated code; choose another name", name, ident)
		}
	}
	if parent != "" && !pascalRe.MatchString(parent) {
		return nil, fmt.Errorf("--parent %q must be a PascalCase model name", parent)
	}
	if movable && parent == "" {
		return nil, errors.New("--movable only applies with --parent")
	}
	if parent == name {
		return nil, errors.New("a resource cannot be its own parent")
	}
	if len(specs) == 0 {
		specs = []string{"name:string"}
	}

	parentColumn := ""
	goNames := map[string]bool{"ID": true, "CreatedAt": true, "UpdatedAt": true}
	if parent == "" {
		goNames["UserID"], goNames["User"] = true, true
	} else {
		parentColumn = naming.Snake(parent) + "_id"
		goNames[naming.GoField(parentColumn)], goNames[parent] = true, true
	}
	claim := func(goName string) error {
		if goNames[goName] {
			return fmt.Errorf("the Go name %s is used twice; rename a field", goName)
		}
		goNames[goName] = true
		return nil
	}

	columns := map[string]bool{}
	for _, spec := range specs {
		f, err := ParseField(spec)
		if err != nil {
			return nil, err
		}
		if f.Column() == parentColumn {
			return nil, fmt.Errorf("field %q: %s is the parent's column, added by --parent", spec, parentColumn)
		}
		if columns[f.Column()] {
			return nil, fmt.Errorf("field %q is declared twice", f.Column())
		}
		columns[f.Column()] = true
		if err := claim(f.GoName()); err != nil {
			return nil, err
		}
		if f.Type == Ref {
			if err := claim(naming.GoField(f.Name)); err != nil { // the belongs-to relation
				return nil, err
			}
		}
		r.Fields = append(r.Fields, f)
	}
	return r, nil
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: parse the resource name and field specs`, plus the trailer.

---

### Task 4: Index the project's Bun models

**Files:**
- Create: `internal/generate/resource/models.go`, `internal/generate/resource/models_test.go`
- Create the fixture sources in `internal/generate/resource/testdata/models/`: `user.go`, `division.go`, `course.go`, `outcome.go`, `unit.go`, `lesson.go`, `link.go`, `tag.go`, `lecture_group.go`, `requests.go`. They are parsed only, never compiled.

**Interfaces:**
- Produces:

```go
type ModelField struct {
	GoName string // CourseID
	GoType string // int64, *int64, string, time.Time, UnitType, …
	Column string // course_id; "" for relations and bun:"-"
}
type Model struct {
	Name      string            // Course
	Table     string            // courses
	Fields    []ModelField
	BelongsTo map[string]string // join column → target model, from rel:belongs-to
}
func (m *Model) HasColumn(col string) bool
func (m *Model) Field(goName string) (ModelField, bool)
type ModelIndex map[string]*Model
func LoadModels(dir string) (ModelIndex, error)
func (idx ModelIndex) FKTarget(m *Model, col string) (*Model, bool)
```

- [ ] **Step 1: Write the fixtures.** All of them are `package models`.

`testdata/models/user.go`:

```go
package models

import (
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:users"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	Username  string    `bun:"username,notnull,unique" json:"username"`
	Password  string    `bun:"password,notnull" json:"-"`
	Email     string    `bun:"email,notnull,unique" json:"email"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
}
```

`testdata/models/division.go` (reference data):

```go
package models

import "github.com/uptrace/bun"

type Division struct {
	bun.BaseModel `bun:"table:divisions,alias:divisions"`

	ID   int64  `bun:"id,pk,autoincrement" json:"id"`
	Name string `bun:"name,notnull,unique" json:"name"`
}
```

`testdata/models/course.go` (flat and owned, with an FK to reference data):

```go
package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Course struct {
	bun.BaseModel `bun:"table:courses,alias:courses"`

	ID         int64     `bun:"id,pk,autoincrement" json:"id"`
	UserID     int64     `bun:"user_id,notnull" json:"-"`
	DivisionID int64     `bun:"division_id,notnull" json:"divisionId"`
	Name       string    `bun:"name,notnull" json:"name"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt  time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updatedAt"`

	User     *User     `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	Division *Division `bun:"rel:belongs-to,join:division_id=id" json:"division,omitempty"`
}
```

`testdata/models/outcome.go`:

```go
package models

import "github.com/uptrace/bun"

type Outcome struct {
	bun.BaseModel `bun:"table:outcomes,alias:outcomes"`

	ID       int64  `bun:"id,pk,autoincrement" json:"id"`
	CourseID int64  `bun:"course_id,notnull" json:"courseId"`
	Name     string `bun:"name,notnull" json:"name"`

	Course *Course `bun:"rel:belongs-to,join:course_id=id" json:"-"`
}
```

`testdata/models/unit.go`. There's no relation field, so `outcome_id` resolves by name, and an empty bun name falls back to the field name:

```go
package models

import "github.com/uptrace/bun"

type UnitType string

type Unit struct {
	bun.BaseModel `bun:"table:units,alias:units"`

	ID        int64    `bun:"id,pk,autoincrement" json:"id"`
	OutcomeID int64    `bun:"outcome_id,notnull" json:"outcomeId"`
	Type      UnitType `bun:"type,notnull" json:"type"`
	Title     string   `bun:",notnull" json:"title"`
}
```

`testdata/models/lesson.go` (a child that also references shared data, Review Focus 2):

```go
package models

import "github.com/uptrace/bun"

type Lesson struct {
	bun.BaseModel `bun:"table:lessons,alias:lessons"`

	ID         int64 `bun:"id,pk,autoincrement" json:"id"`
	OutcomeID  int64 `bun:"outcome_id,notnull" json:"outcomeId"`
	DivisionID int64 `bun:"division_id,notnull" json:"divisionId"`
}
```

`testdata/models/link.go` (two owned parents, so its chain is ambiguous):

```go
package models

import "github.com/uptrace/bun"

type Link struct {
	bun.BaseModel `bun:"table:links,alias:links"`

	ID        int64 `bun:"id,pk,autoincrement" json:"id"`
	CourseID  int64 `bun:"course_id,notnull" json:"courseId"`
	OutcomeID int64 `bun:"outcome_id,notnull" json:"outcomeId"`
}
```

`testdata/models/tag.go` (a grouped type declaration):

```go
package models

import "github.com/uptrace/bun"

type (
	Tags = []Tag
	Tag  struct {
		bun.BaseModel `bun:"table:tags,alias:tags"`

		ID    int64  `bun:"id,pk,autoincrement" json:"id"`
		Label string `bun:"label,notnull" json:"label"`
	}
)
```

`testdata/models/lecture_group.go`:

```go
package models

import "github.com/uptrace/bun"

type LectureGroup struct {
	bun.BaseModel `bun:"table:lecture_groups,alias:lecture_groups"`

	ID     int64  `bun:"id,pk,autoincrement" json:"id"`
	UserID int64  `bun:"user_id,notnull" json:"-"`
	Name   string `bun:"name,notnull" json:"name"`
}
```

`testdata/models/requests.go` (not a model):

```go
package models

type CourseRequest struct {
	Name string `json:"name"`
}
```

- [ ] **Step 2: Write the failing tests** (`models_test.go`):

```go
package resource

import "testing"

// loadFixtureModels indexes testdata/models, shared by the model, ownership and view tests.
func loadFixtureModels(t *testing.T) ModelIndex {
	t.Helper()
	idx, err := LoadModels("testdata/models")
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestLoadModels(t *testing.T) {
	idx := loadFixtureModels(t)

	course := idx["Course"]
	if course == nil || course.Table != "courses" {
		t.Fatalf("Course: %+v", course)
	}
	if !course.HasColumn("user_id") || !course.HasColumn("division_id") || course.HasColumn("division") {
		t.Errorf("Course columns: %+v", course.Fields)
	}
	if course.BelongsTo["division_id"] != "Division" {
		t.Errorf("belongs-to relations: %+v", course.BelongsTo)
	}
	if f, ok := course.Field("DivisionID"); !ok || f.GoType != "int64" || f.Column != "division_id" {
		t.Errorf("Field(DivisionID) = %+v, %v", f, ok)
	}

	unit := idx["Unit"]
	if !unit.HasColumn("title") {
		t.Error("an empty bun name must fall back to the snake-cased field name")
	}
	if target, ok := idx.FKTarget(unit, "outcome_id"); !ok || target.Name != "Outcome" {
		t.Error("outcome_id without a relation must resolve to Outcome by name")
	}
	if _, ok := idx["CourseRequest"]; ok {
		t.Error("a struct without bun.BaseModel is not a model")
	}
	if idx["Tag"] == nil || idx["LectureGroup"].Table != "lecture_groups" {
		t.Error("grouped declarations and multiword models must be indexed")
	}
}

func TestLoadModelsWithoutADirectory(t *testing.T) {
	idx, err := LoadModels("testdata/nope")
	if err != nil || len(idx) != 0 {
		t.Fatalf("a project without app/models has no models: %v %v", idx, err)
	}
}
```

- [ ] **Step 3:** Run `go test ./internal/generate/resource/ -run LoadModels`. Expected: FAIL (undefined `LoadModels`).
- [ ] **Step 4: Implement** `models.go`:

```go
package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// ModelField is one struct field of a Bun model, as far as the generator needs it.
type ModelField struct {
	GoName string // CourseID
	GoType string // int64, *int64, string, time.Time, UnitType, …
	Column string // course_id; "" for relations and bun:"-"
}

// Model is a struct in app/models that embeds bun.BaseModel.
type Model struct {
	Name      string            // Course
	Table     string            // courses
	Fields    []ModelField
	BelongsTo map[string]string // join column → target model, from rel:belongs-to
}

func (m *Model) HasColumn(col string) bool {
	for _, f := range m.Fields {
		if f.Column == col {
			return true
		}
	}
	return false
}

func (m *Model) Field(goName string) (ModelField, bool) {
	for _, f := range m.Fields {
		if f.GoName == goName {
			return f, true
		}
	}
	return ModelField{}, false
}

// ModelIndex maps model names to models.
type ModelIndex map[string]*Model

// LoadModels parses every non-test Go file in dir. A missing directory is an empty index.
func LoadModels(dir string) (ModelIndex, error) {
	idx := ModelIndex{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return idx, nil
	}
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filepath.Join(dir, name), err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts := spec.(*ast.TypeSpec)
				if st, ok := ts.Type.(*ast.StructType); ok {
					if m := modelFromStruct(ts.Name.Name, st); m != nil {
						idx[m.Name] = m
					}
				}
			}
		}
	}
	return idx, nil
}

func modelFromStruct(name string, st *ast.StructType) *Model {
	m := &Model{Name: name, BelongsTo: map[string]string{}}
	isModel := false
	for _, f := range st.Fields.List {
		tag := bunTag(f.Tag)
		if len(f.Names) == 0 {
			if sel, ok := f.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "BaseModel" {
				isModel = true
				for _, part := range strings.Split(tag, ",") {
					if table, ok := strings.CutPrefix(part, "table:"); ok {
						m.Table = table
					}
				}
			}
			continue
		}
		goType := types.ExprString(f.Type)
		for _, n := range f.Names {
			mf := ModelField{GoName: n.Name, GoType: goType}
			switch col, _, _ := strings.Cut(tag, ","); {
			case strings.Contains(tag, "rel:"):
				if strings.Contains(tag, "rel:belongs-to") {
					for _, part := range strings.Split(tag, ",") {
						if join, ok := strings.CutPrefix(part, "join:"); ok {
							joinCol, _, _ := strings.Cut(join, "=")
							m.BelongsTo[joinCol] = strings.TrimPrefix(goType, "*")
						}
					}
				}
			case col == "-":
			case col == "":
				mf.Column = naming.Snake(n.Name) // Bun's default column name
			default:
				mf.Column = col
			}
			m.Fields = append(m.Fields, mf)
		}
	}
	if !isModel {
		return nil
	}
	if m.Table == "" {
		m.Table = naming.Snake(naming.Plural(name)) // Bun's default table name
	}
	return m
}

func bunTag(lit *ast.BasicLit) string {
	if lit == nil {
		return ""
	}
	raw, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return reflect.StructTag(raw).Get("bun")
}

// FKTarget reports the model an x_id column points at: the belongs-to relation that joins on
// it, or else the model its name implies (course_id → Course).
func (idx ModelIndex) FKTarget(m *Model, col string) (*Model, bool) {
	if !strings.HasSuffix(col, "_id") {
		return nil, false
	}
	name, ok := m.BelongsTo[col]
	if !ok {
		name = naming.GoField(strings.TrimSuffix(col, "_id"))
	}
	target, ok := idx[name]
	return target, ok
}
```

- [ ] **Step 5:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 6:** Commit with the message `resource: index the project's Bun models`, plus the trailer.

---

### Task 5: Derive ownership chains and predicates

**Files:** Create `internal/generate/resource/ownership.go` and `internal/generate/resource/ownership_test.go`.

**Interfaces:**
- Consumes: `ModelIndex`, `Model.HasColumn`, `ModelIndex.FKTarget` (Task 4).
- Produces:

```go
type Chain struct {
	Models []*Model // the model itself first, the root-owned model last
	FKs    []string // FKs[i] is the column on Models[i] that points at Models[i+1]
}
func (c Chain) Root() *Model
func (c Chain) Predicate(childTable, parentFK string) string
func (idx ModelIndex) IsOwned(name string) bool
func (idx ModelIndex) OwnerChain(name string) (Chain, error)
```

- [ ] **Step 1: Write the failing tests** (`ownership_test.go`):

```go
package resource

import (
	"strings"
	"testing"
)

func TestIsOwned(t *testing.T) {
	idx := loadFixtureModels(t)
	for name, want := range map[string]bool{
		"Course": true, "Outcome": true, "Unit": true, "Lesson": true, "LectureGroup": true,
		"Division": false, "User": false, "Tag": false, "Missing": false,
	} {
		if got := idx.IsOwned(name); got != want {
			t.Errorf("IsOwned(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestOwnerChainPredicate(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct{ parent, childTable, childFK, want string }{
		{"Course", "outcomes", "course_id",
			"outcomes.course_id IN (SELECT id FROM courses WHERE user_id = ?)"},
		{"Outcome", "units", "outcome_id",
			"units.outcome_id IN (\n\tSELECT outcomes.id FROM outcomes\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
		{"Unit", "records", "unit_id",
			"records.unit_id IN (\n\tSELECT units.id FROM units\n\tJOIN outcomes ON outcomes.id = units.outcome_id\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
		// Review Focus 2: Lesson's division_id points at reference data and is not a parent.
		{"Lesson", "notes", "lesson_id",
			"notes.lesson_id IN (\n\tSELECT lessons.id FROM lessons\n\tJOIN outcomes ON outcomes.id = lessons.outcome_id\n\tJOIN courses ON courses.id = outcomes.course_id\n\tWHERE courses.user_id = ?\n)"},
	}
	for _, tt := range tests {
		chain, err := idx.OwnerChain(tt.parent)
		if err != nil {
			t.Errorf("OwnerChain(%s): %v", tt.parent, err)
			continue
		}
		if got := chain.Predicate(tt.childTable, tt.childFK); got != tt.want {
			t.Errorf("predicate under %s:\n got %q\nwant %q", tt.parent, got, tt.want)
		}
	}
}

func TestOwnerChainErrors(t *testing.T) {
	idx := loadFixtureModels(t)
	for name, want := range map[string]string{
		"Division": "not owned by a user",
		"Link":     "more than one owned parent (course_id, outcome_id)",
		"Missing":  "not found in app/models",
	} {
		_, err := idx.OwnerChain(name)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("OwnerChain(%s) error = %v, want it to mention %q", name, err, want)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run 'Owned|OwnerChain'`. Expected: FAIL (undefined `IsOwned`).
- [ ] **Step 3: Implement** `ownership.go`:

```go
package resource

import (
	"fmt"
	"strings"
)

// Chain is the path from a model up to the row that carries user_id.
type Chain struct {
	Models []*Model // the model itself first, the root-owned model last
	FKs    []string // FKs[i] is the column on Models[i] that points at Models[i+1]
}

func (c Chain) Root() *Model { return c.Models[len(c.Models)-1] }

// Predicate is the ownership condition for a child table whose parentFK column points at the
// chain's first model, in the form patterns.md uses: a subquery when that model carries
// user_id itself, and a JOIN chain when it is deeper.
func (c Chain) Predicate(childTable, parentFK string) string {
	parent := c.Models[0]
	if len(c.Models) == 1 {
		return fmt.Sprintf("%s.%s IN (SELECT id FROM %s WHERE user_id = ?)", childTable, parentFK, parent.Table)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s.%s IN (\n\tSELECT %s.id FROM %s\n", childTable, parentFK, parent.Table, parent.Table)
	for i := 1; i < len(c.Models); i++ {
		prev, cur := c.Models[i-1], c.Models[i]
		fmt.Fprintf(&b, "\tJOIN %s ON %s.id = %s.%s\n", cur.Table, cur.Table, prev.Table, c.FKs[i-1])
	}
	fmt.Fprintf(&b, "\tWHERE %s.user_id = ?\n)", c.Root().Table)
	return b.String()
}

// IsOwned reports whether rows of the named model belong to a user, directly through a user_id
// column or through a parent.
func (idx ModelIndex) IsOwned(name string) bool {
	m, ok := idx[name]
	return ok && idx.owned(m, map[string]bool{})
}

func (idx ModelIndex) owned(m *Model, visiting map[string]bool) bool {
	if m.HasColumn("user_id") {
		return true
	}
	if visiting[m.Name] {
		return false
	}
	visiting[m.Name] = true
	defer delete(visiting, m.Name)
	return len(idx.ownedParents(m, visiting)) > 0
}

type fk struct {
	column string
	target *Model
}

func (idx ModelIndex) ownedParents(m *Model, visiting map[string]bool) []fk {
	var out []fk
	for _, f := range m.Fields {
		if f.Column == "" {
			continue
		}
		if target, ok := idx.FKTarget(m, f.Column); ok && target.Name != m.Name && idx.owned(target, visiting) {
			out = append(out, fk{f.Column, target})
		}
	}
	return out
}

// OwnerChain walks from the named model up to the model with a user_id column. It fails when the
// model is not owned, or when a model on the way has more than one owned parent.
func (idx ModelIndex) OwnerChain(name string) (Chain, error) {
	m, ok := idx[name]
	if !ok {
		return Chain{}, fmt.Errorf("model %s not found in app/models", name)
	}
	var c Chain
	seen := map[string]bool{}
	for {
		if seen[m.Name] {
			return Chain{}, fmt.Errorf("the ownership chain of %s loops back to %s", name, m.Name)
		}
		seen[m.Name] = true
		c.Models = append(c.Models, m)
		if m.HasColumn("user_id") {
			return c, nil
		}
		parents := idx.ownedParents(m, map[string]bool{m.Name: true})
		switch len(parents) {
		case 0:
			return Chain{}, fmt.Errorf("model %s is not owned by a user: it has no user_id column and no foreign key to an owned model", m.Name)
		case 1:
		default:
			cols := make([]string, len(parents))
			for i, p := range parents {
				cols[i] = p.column
			}
			return Chain{}, fmt.Errorf("model %s has more than one owned parent (%s); the generator cannot choose one", m.Name, strings.Join(cols, ", "))
		}
		c.FKs = append(c.FKs, parents[0].column)
		m = parents[0].target
	}
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: derive ownership chains and predicates from the models`, plus the trailer.

---

### Task 6: Build the template view

Every name, type, tag, schema, SQL fragment and test sample is computed here, once, so the templates stay plain.

**Files:** Create `internal/generate/resource/view.go` and `internal/generate/resource/view_test.go`.

**Interfaces:**
- Consumes: `Resource`, `Field` (Task 3); `ModelIndex`, `OwnerChain`, `IsOwned`, `Predicate` (Tasks 4–5); `naming.*`.
- Produces `buildView(res *Resource, module string, idx ModelIndex) (*view, error)` and these types, which Tasks 7–13 read:

```go
type view struct {
	Module, Name, Plural, Var, PluralVar, Recv, Table, Route string
	Label, LabelLower, PluralLabelLower                     string
	Parent                                                  *parentView // nil when flat
	Movable                                                 bool
	Fields                                                  []fieldView
	Enums                                                   []enumView
	Refs                                                    []refView
	OrderBy, ChildOrder, PredicateConst, Predicate, Updatable string
	UsesMaxChars, HasRequired, SamplesUseTime               bool
	UpdatedField                                            *fieldView // first required string field, or nil
}
type parentView struct{ Name, Plural, Var, Table, Field, Column, JSON, LabelLower string }
type fieldView struct {
	GoName, GoType, ReqType, JSON, Column, BunTag, Schema, Apply, SQLType, Check string
	Required                                                                      bool
	SampleModel, SampleReq, UpdateSample                                          string
}
type enumView struct {
	Type, SQLType string
	Consts        []enumConst
}
type enumConst struct{ Name, Value string }
type refView struct {
	Relation, Model, Column, Table string
	Optional                       bool
}
```

- [ ] **Step 1: Write the failing tests** (`view_test.go`):

```go
package resource

import (
	"reflect"
	"strings"
	"testing"
)

func mustView(t *testing.T, name string, specs []string, parent string, movable bool) *view {
	t.Helper()
	res, err := NewResource(name, specs, parent, "", movable)
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildView(res, "example.com/shop", loadFixtureModels(t))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestBuildViewFlat(t *testing.T) {
	v := mustView(t, "Seminar", []string{"name:string", "lecture_hours:int", "division:ref", "notes:text"}, "", false)

	if v.Table != "seminars" || v.Route != "seminars" || v.Var != "seminar" || v.PluralVar != "seminars" || v.Recv != "s" {
		t.Fatalf("names: %+v", v)
	}
	if v.Updatable != `"name", "lecture_hours", "division_id", "notes"` {
		t.Errorf("Updatable = %s", v.Updatable)
	}
	if v.OrderBy != `"seminars.name", "seminars.id"` {
		t.Errorf("OrderBy = %s", v.OrderBy)
	}
	if want := []refView{{Relation: "Division", Model: "Division", Column: "division_id", Table: "divisions"}}; !reflect.DeepEqual(v.Refs, want) {
		t.Errorf("Refs = %+v", v.Refs)
	}
	name, hours, division, notes := v.Fields[0], v.Fields[1], v.Fields[2], v.Fields[3]
	if name.Schema != "zog.String().Trim().Min(1).TestFunc(maxChars(150)).Required()" || name.SQLType != "VARCHAR(150) NOT NULL" || !name.Required {
		t.Errorf("name: %+v", name)
	}
	if hours.Schema != "zog.Int().GTE(0)" || hours.Check != "lecture_hours >= 0" || hours.Required || hours.JSON != "lectureHours" {
		t.Errorf("lecture_hours: %+v", hours)
	}
	if division.GoName != "DivisionID" || division.SampleModel != "seedDivision(t).ID" || division.Schema != "zog.Int64().GT(0).Required()" {
		t.Errorf("division: %+v", division)
	}
	if notes.SQLType != "TEXT NOT NULL DEFAULT ''" || notes.Schema != "zog.String()" || notes.Required {
		t.Errorf("notes: %+v", notes)
	}
	if !v.UsesMaxChars || !v.HasRequired || v.UpdatedField == nil || v.UpdatedField.GoName != "Name" {
		t.Errorf("flags: maxChars=%v required=%v updated=%+v", v.UsesMaxChars, v.HasRequired, v.UpdatedField)
	}
}

// Review Focus 4: multiword names agree everywhere.
func TestBuildViewMultiwordChild(t *testing.T) {
	v := mustView(t, "StudyNote", nil, "LectureGroup", false)
	p := v.Parent
	if p.Field != "LectureGroupID" || p.Column != "lecture_group_id" || p.JSON != "lectureGroupId" || p.Plural != "LectureGroups" || p.Var != "lectureGroup" {
		t.Fatalf("parent: %+v", p)
	}
	if v.Table != "study_notes" || v.Route != "study-notes" || v.PredicateConst != "studyNotesOwnedByUser" {
		t.Errorf("names: %s %s %s", v.Table, v.Route, v.PredicateConst)
	}
	if v.Predicate != "study_notes.lecture_group_id IN (SELECT id FROM lecture_groups WHERE user_id = ?)" {
		t.Errorf("Predicate = %s", v.Predicate)
	}
	if v.ChildOrder != `"study_notes.lecture_group_id", "study_notes.name", "study_notes.id"` || v.Updatable != `"name"` {
		t.Errorf("order %s, updatable %s", v.ChildOrder, v.Updatable)
	}
}

func TestBuildViewMovableEnumOptional(t *testing.T) {
	v := mustView(t, "Slot", []string{"kind:enum:lecture,in_person", "starts_at:time:optional", "active:bool"}, "Outcome", true)
	if v.Updatable != `"outcome_id", "kind", "starts_at", "active"` {
		t.Errorf("Updatable = %s", v.Updatable)
	}
	want := enumView{Type: "SlotKind", SQLType: "slot_kind", Consts: []enumConst{{"SlotKindLecture", "lecture"}, {"SlotKindInPerson", "in_person"}}}
	if len(v.Enums) != 1 || !reflect.DeepEqual(v.Enums[0], want) {
		t.Errorf("Enums = %+v", v.Enums)
	}
	kind, startsAt, active := v.Fields[0], v.Fields[1], v.Fields[2]
	if kind.Apply != "SlotKind(r.Kind)" || kind.ReqType != "string" || kind.GoType != "SlotKind" || kind.SampleReq != "string(models.SlotKindLecture)" || kind.SampleModel != "models.SlotKindLecture" {
		t.Errorf("kind: %+v", kind)
	}
	if startsAt.GoType != "*time.Time" || startsAt.Schema != "zog.Ptr(zog.Time())" || startsAt.BunTag != "starts_at" || startsAt.SampleModel != "" {
		t.Errorf("starts_at: %+v", startsAt)
	}
	if active.SQLType != "BOOLEAN NOT NULL DEFAULT false" || active.SampleReq != "true" {
		t.Errorf("active: %+v", active)
	}
	if v.UpdatedField != nil || v.UsesMaxChars {
		t.Errorf("no string field: updated=%+v maxChars=%v", v.UpdatedField, v.UsesMaxChars)
	}
}

func TestBuildViewErrors(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct {
		name, parent string
		specs        []string
		want         string
	}{
		{"Seminar", "", []string{"course:ref"}, "Course is owned by a user; use --parent Course"},
		{"Seminar", "", []string{"thing:ref"}, "model Thing not found"},
		{"Seminar", "Division", nil, "not owned by a user"},
	}
	for _, tt := range tests {
		res, err := NewResource(tt.name, tt.specs, tt.parent, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := buildView(res, "example.com/shop", idx); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%v under %q: error = %v, want it to mention %q", tt.specs, tt.parent, err, tt.want)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run BuildView`. Expected: FAIL (undefined `buildView`).
- [ ] **Step 3: Implement** `view.go`:

```go
package resource

import (
	"fmt"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// view is the data every template renders from.
type view struct {
	Module           string
	Name             string // Course
	Plural           string // Courses
	Var              string // course
	PluralVar        string // courses
	Recv             string // mapper receiver: the name's first letter, or m where that is r
	Table            string // courses
	Route            string // courses; kebab-case for multiword names
	Label            string // Course, Lecture group
	LabelLower       string // course, lecture group
	PluralLabelLower string // courses, lecture groups
	Parent           *parentView
	Movable          bool
	Fields           []fieldView
	Enums            []enumView
	Refs             []refView
	OrderBy          string // Order() arguments for a list
	ChildOrder       string // Order() arguments for a child's unfiltered list
	PredicateConst   string
	Predicate        string
	Updatable        string // the XUpdatableColumns elements
	UsesMaxChars     bool
	HasRequired      bool
	SamplesUseTime   bool
	UpdatedField     *fieldView
}

type parentView struct {
	Name, Plural, Var, Table string // Course, Courses, course, courses
	Field, Column, JSON      string // CourseID, course_id, courseId
	LabelLower               string
}

type fieldView struct {
	GoName, GoType, ReqType, JSON, Column string
	BunTag, Schema, Apply, SQLType, Check string
	Required                              bool   // the schema calls Required()
	SampleModel                           string // Go expression in a seed; "" leaves the zero value
	SampleReq                             string // Go expression in a test request
	UpdateSample                          string // string fields: the value an update test sends
}

type enumView struct {
	Type    string // SlotKind
	SQLType string // slot_kind
	Consts  []enumConst
}

type enumConst struct{ Name, Value string }

type refView struct {
	Relation string // Division: the belongs-to field
	Model    string
	Column   string
	Table    string
	Optional bool
}

func buildView(res *Resource, module string, idx ModelIndex) (*view, error) {
	v := &view{
		Module:           module,
		Name:             res.Name,
		Plural:           res.Plural,
		Var:              naming.Var(res.Name),
		PluralVar:        naming.Var(res.Plural),
		Table:            naming.Snake(res.Plural),
		Label:            naming.Label(res.Name),
		LabelLower:       naming.LabelLower(res.Name),
		PluralLabelLower: naming.LabelLower(res.Plural),
		Movable:          res.Movable,
		Recv:             strings.ToLower(res.Name[:1]),
	}
	v.Route = naming.Kebab(v.Table)
	if v.Recv == "r" {
		v.Recv = "m" // r is the request receiver
	}

	var updatable []string
	if res.Parent != "" {
		chain, err := idx.OwnerChain(res.Parent)
		if err != nil {
			return nil, fmt.Errorf("--parent %s: %w", res.Parent, err)
		}
		pm := chain.Models[0]
		column := naming.Snake(pm.Name) + "_id"
		v.Parent = &parentView{
			Name: pm.Name, Plural: naming.GoField(pm.Table), Var: naming.Var(pm.Name), Table: pm.Table,
			Field: naming.GoField(column), Column: column, JSON: naming.Camel(column),
			LabelLower: naming.LabelLower(pm.Name),
		}
		v.PredicateConst = v.PluralVar + "OwnedByUser"
		v.Predicate = chain.Predicate(v.Table, column)
		v.HasRequired = true // the parent FK is required
		if res.Movable {
			updatable = append(updatable, column)
		}
	}

	orderKey := ""
	for _, f := range res.Fields {
		fv, err := v.field(f, idx)
		if err != nil {
			return nil, err
		}
		v.HasRequired = v.HasRequired || fv.Required
		if f.Type == String && !f.Optional && orderKey == "" {
			orderKey = f.Column()
		}
		updatable = append(updatable, f.Column())
		v.Fields = append(v.Fields, fv)
	}
	for i, f := range res.Fields {
		if f.Type == String && !f.Optional {
			v.UpdatedField = &v.Fields[i]
			break
		}
	}

	v.Updatable = quoteJoin(updatable)
	var order []string
	if orderKey != "" {
		order = append(order, v.Table+"."+orderKey)
	}
	order = append(order, v.Table+".id")
	v.OrderBy = quoteJoin(order)
	if v.Parent != nil {
		v.ChildOrder = quoteJoin(append([]string{v.Table + "." + v.Parent.Column}, order...))
	}
	return v, nil
}

func (v *view) field(f Field, idx ModelIndex) (fieldView, error) {
	col := f.Column()
	fv := fieldView{GoName: f.GoName(), JSON: f.JSON(), Column: col, Apply: "r." + f.GoName()}
	required := !f.Optional
	switch f.Type {
	case String:
		fv.GoType, fv.ReqType = "string", "string"
		v.UsesMaxChars = true
		if required {
			fv.BunTag, fv.SQLType = col+",notnull", fmt.Sprintf("VARCHAR(%d) NOT NULL", f.Length)
			fv.Schema = fmt.Sprintf("zog.String().Trim().Min(1).TestFunc(maxChars(%d)).Required()", f.Length)
			fv.Required = true
			fv.SampleModel = fmt.Sprintf("%q", truncate("Sample", f.Length))
			fv.SampleReq = fv.SampleModel
			fv.UpdateSample = fmt.Sprintf("%q", truncate("Updated", f.Length))
		} else {
			fv.BunTag, fv.SQLType = col+",nullzero", fmt.Sprintf("VARCHAR(%d)", f.Length)
			fv.Schema = fmt.Sprintf("zog.String().Trim().TestFunc(maxChars(%d))", f.Length)
		}
	case Text:
		fv.GoType, fv.ReqType, fv.BunTag = "string", "string", col+",notnull"
		fv.SQLType, fv.Schema = "TEXT NOT NULL DEFAULT ''", "zog.String()"
		fv.SampleModel, fv.SampleReq = `"Sample text"`, `"Sample text"`
	case Int, Int64:
		goType, sqlType, zogType := "int", "INTEGER", "Int"
		if f.Type == Int64 {
			goType, sqlType, zogType = "int64", "BIGINT", "Int64"
		}
		fv.Check = col + " >= 0" // 0 is valid, so the schema never calls Required
		if required {
			fv.GoType, fv.BunTag, fv.SQLType = goType, col+",notnull", sqlType+" NOT NULL DEFAULT 0"
			fv.Schema = fmt.Sprintf("zog.%s().GTE(0)", zogType)
			fv.SampleModel, fv.SampleReq = "1", "1"
		} else {
			fv.GoType, fv.BunTag, fv.SQLType = "*"+goType, col, sqlType
			fv.Schema = fmt.Sprintf("zog.Ptr(zog.%s().GTE(0))", zogType)
		}
		fv.ReqType = fv.GoType
	case Bool:
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "bool", col+",notnull", "BOOLEAN NOT NULL DEFAULT false", "zog.Bool()"
			fv.SampleModel, fv.SampleReq = "true", "true"
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*bool", col, "BOOLEAN", "zog.Ptr(zog.Bool())"
		}
		fv.ReqType = fv.GoType
	case Time:
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "time.Time", col+",notnull", "TIMESTAMPTZ NOT NULL", "zog.Time().Required()"
			fv.Required = true
			fv.SampleModel = "time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)"
			fv.SampleReq = fv.SampleModel
			v.SamplesUseTime = true
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*time.Time", col, "TIMESTAMPTZ", "zog.Ptr(zog.Time())"
		}
		fv.ReqType = fv.GoType
	case Enum:
		e := enumView{Type: v.Name + f.GoName(), SQLType: naming.Snake(v.Name) + "_" + col}
		for _, value := range f.EnumValues {
			e.Consts = append(e.Consts, enumConst{Name: e.Type + naming.GoField(value), Value: value})
		}
		v.Enums = append(v.Enums, e)
		fv.GoType, fv.ReqType, fv.Apply = e.Type, "string", e.Type+"(r."+f.GoName()+")"
		if required {
			fv.BunTag, fv.SQLType = col+",notnull", e.SQLType+" NOT NULL"
			fv.Schema = fmt.Sprintf("zog.String().OneOf(%sValues).Required()", e.Type)
			fv.Required = true
			fv.SampleModel = "models." + e.Consts[0].Name
			fv.SampleReq = "string(models." + e.Consts[0].Name + ")"
		} else {
			fv.BunTag, fv.SQLType = col+",nullzero", e.SQLType
			fv.Schema = fmt.Sprintf("zog.String().OneOf(%sValues)", e.Type)
		}
	case Ref:
		target, ok := idx[f.RefModel]
		if !ok {
			return fieldView{}, fmt.Errorf("field %s: model %s not found in app/models", f.Name, f.RefModel)
		}
		if idx.IsOwned(target.Name) {
			return fieldView{}, fmt.Errorf("field %s: %s is owned by a user; use --parent %s, or the ownership check would be skipped", f.Name, target.Name, target.Name)
		}
		v.Refs = append(v.Refs, refView{Relation: naming.GoField(f.Name), Model: target.Name, Column: col, Table: target.Table, Optional: f.Optional})
		if required {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "int64", col+",notnull", "BIGINT NOT NULL", "zog.Int64().GT(0).Required()"
			fv.Required = true
			fv.SampleModel = "seed" + target.Name + "(t).ID"
			fv.SampleReq = fv.SampleModel
		} else {
			fv.GoType, fv.BunTag, fv.SQLType, fv.Schema = "*int64", col, "BIGINT", "zog.Ptr(zog.Int64().GT(0))"
		}
		fv.ReqType = fv.GoType
	}
	return fv, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: build the template view from the spec and the models`, plus the trailer.

---

### Task 7: Render the model and the migration

**Files:**
- Create: `internal/generate/resource/render.go`, `internal/generate/resource/render_test.go`, `internal/generate/resource/templates/model.go.tmpl`

**Interfaces:**
- Consumes: `view` (Task 6).
- Produces: `renderGo(name string, data any) (string, error)`, `renderMigration(v *view) string`, `const setupMigration string`, and the test helpers `mustRender(t, name, data) string` and `loose(t, src, want)`.

- [ ] **Step 1: Write the failing tests** (`render_test.go`):

```go
package resource

import (
	"regexp"
	"strings"
	"testing"
)

// loose fails unless src contains want, where any run of spaces in want matches any run of
// whitespace in src, so gofmt's column alignment doesn't matter.
func loose(t *testing.T, src, want string) {
	t.Helper()
	parts := strings.Fields(want)
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	if !regexp.MustCompile(strings.Join(parts, `\s+`)).MatchString(src) {
		t.Errorf("missing %q in:\n%s", want, src)
	}
}

func mustRender(t *testing.T, name string, data any) string {
	t.Helper()
	src, err := renderGo(name, data)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestRenderModelFlat(t *testing.T) {
	src := mustRender(t, "model.go.tmpl", mustView(t, "Seminar", []string{"name:string", "lecture_hours:int", "division:ref"}, "", false))
	for _, want := range []string{
		"type ( Seminars = []Seminar Seminar struct {",
		"bun.BaseModel `bun:\"table:seminars,alias:seminars\"`",
		"UserID int64 `bun:\"user_id,notnull\" json:\"-\"`",
		"Name string `bun:\"name,notnull\" json:\"name\"`",
		"LectureHours int `bun:\"lecture_hours,notnull\" json:\"lectureHours\"`",
		"DivisionID int64 `bun:\"division_id,notnull\" json:\"divisionId\"`",
		"User *User `bun:\"rel:belongs-to,join:user_id=id\" json:\"-\"`",
		"Division *Division `bun:\"rel:belongs-to,join:division_id=id\" json:\"-\"`",
		"type SeminarRequest struct { Name string `json:\"name\"` LectureHours int `json:\"lectureHours\"` DivisionID int64 `json:\"divisionId\"` }",
		"\"Name\": zog.String().Trim().Min(1).TestFunc(maxChars(150)).Required(),",
		"\"LectureHours\": zog.Int().GTE(0),",
		"\"DivisionID\": zog.Int64().GT(0).Required(),",
		"func (r SeminarRequest) ToModel(userID int64) *Seminar { s := &Seminar{UserID: userID} r.ApplyTo(s) return s }",
		"func (r SeminarRequest) ApplyTo(s *Seminar) { s.Name = r.Name s.LectureHours = r.LectureHours s.DivisionID = r.DivisionID }",
		"var SeminarUpdatableColumns = []string{\"name\", \"lecture_hours\", \"division_id\"}",
		"func NewSeminarResponse(s *Seminar) SeminarResponse {",
		"res[i] = NewSeminarResponse(&seminars[i])",
	} {
		loose(t, src, want)
	}
}

func TestRenderModelChildImmutable(t *testing.T) {
	src := mustRender(t, "model.go.tmpl", mustView(t, "Topic", []string{"name:string"}, "Course", false))
	for _, want := range []string{
		"CourseID int64 `bun:\"course_id,notnull\" json:\"courseId\"`",
		"Course *Course `bun:\"rel:belongs-to,join:course_id=id\" json:\"-\"`",
		"type TopicRequest struct { CourseID int64 `json:\"courseId\"` Name string `json:\"name\"` }",
		"\"CourseID\": zog.Int64().GT(0).Required(),",
		"func (r TopicRequest) ToModel() *Topic { t := &Topic{CourseID: r.CourseID} r.ApplyTo(t) return t }",
		"func (r TopicRequest) ApplyTo(t *Topic) { t.Name = r.Name }",
		"var TopicUpdatableColumns = []string{\"name\"}",
		"CourseID: t.CourseID,",
	} {
		loose(t, src, want)
	}
	if strings.Contains(src, "UserID") {
		t.Error("a child has no owner column of its own")
	}
}

func TestRenderModelEnumMovable(t *testing.T) {
	src := mustRender(t, "model.go.tmpl", mustView(t, "Slot", []string{"kind:enum:lecture,lab", "starts_at:time:optional"}, "Outcome", true))
	for _, want := range []string{
		"type SlotKind string",
		"SlotKindLecture SlotKind = \"lecture\"",
		"var SlotKindValues = []string{string(SlotKindLecture), string(SlotKindLab)}",
		"Kind SlotKind `bun:\"kind,notnull\" json:\"kind\"`",
		"StartsAt *time.Time `bun:\"starts_at\" json:\"startsAt\"`",
		"Kind string `json:\"kind\"`",
		"\"Kind\": zog.String().OneOf(SlotKindValues).Required(),",
		"\"StartsAt\": zog.Ptr(zog.Time()),",
		"func (r SlotRequest) ToModel() *Slot { s := &Slot{} r.ApplyTo(s) return s }",
		"s.OutcomeID = r.OutcomeID // re-parentable: the service verifies the destination",
		"s.Kind = SlotKind(r.Kind)",
		"var SlotUpdatableColumns = []string{\"outcome_id\", \"kind\", \"starts_at\"}",
	} {
		loose(t, src, want)
	}
}

func TestRenderMigrationFlat(t *testing.T) {
	sql := renderMigration(mustView(t, "Seminar", []string{"name:string", "lecture_hours:int", "division:ref"}, "", false))
	for _, want := range []string{
		"-- +goose Up\nCREATE TABLE seminars (\n",
		"id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,",
		"user_id BIGINT NOT NULL,",
		"name VARCHAR(150) NOT NULL,",
		"lecture_hours INTEGER NOT NULL DEFAULT 0,",
		"division_id BIGINT NOT NULL,",
		"updated_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,",
		"CONSTRAINT seminars_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,",
		"CONSTRAINT seminars_division_id_fkey FOREIGN KEY (division_id) REFERENCES divisions (id) ON DELETE RESTRICT,",
		"CONSTRAINT seminars_lecture_hours_check CHECK (lecture_hours >= 0)\n);",
		"CREATE INDEX seminars_user_id_idx ON seminars (user_id);",
		"CREATE INDEX seminars_division_id_idx ON seminars (division_id);",
		"CREATE TRIGGER seminars_set_updated_at BEFORE UPDATE ON seminars FOR EACH ROW EXECUTE FUNCTION set_updated_at();",
		"-- +goose Down\nDROP TABLE IF EXISTS seminars;\n",
	} {
		loose(t, sql, want)
	}
}

func TestRenderMigrationEnumChild(t *testing.T) {
	sql := renderMigration(mustView(t, "Slot", []string{"kind:enum:lecture,lab", "starts_at:time:optional"}, "Outcome", true))
	for _, want := range []string{
		"-- +goose Up\nCREATE TYPE slot_kind AS ENUM ('lecture', 'lab');",
		"outcome_id BIGINT NOT NULL,",
		"kind slot_kind NOT NULL,",
		"starts_at TIMESTAMPTZ,",
		"CONSTRAINT slots_outcome_id_fkey FOREIGN KEY (outcome_id) REFERENCES outcomes (id) ON DELETE CASCADE\n);",
		"CREATE INDEX slots_outcome_id_idx ON slots (outcome_id);",
		"DROP TABLE IF EXISTS slots;\nDROP TYPE IF EXISTS slot_kind;",
	} {
		loose(t, sql, want)
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run Render`. Expected: FAIL (undefined `renderGo`).
- [ ] **Step 3: Implement.** `render.go`:

```go
package resource

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.New("").ParseFS(templateFS, "templates/*.tmpl"))

// renderGo executes a Go template and gofmt's the result, so alignment is never the template's
// job and a template bug surfaces as a parse error naming the template.
func renderGo(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("%s produced invalid Go: %w\n%s", name, err, buf.String())
	}
	return string(src), nil
}

// setupMigration creates the updated_at trigger function every generated table uses.
const setupMigration = `-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = current_timestamp;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- set_updated_at() stays: the tables of later migrations depend on it.
`

// renderMigration writes the database.md table template: the table with its foreign keys and
// CHECKs, an index per foreign key, and the updated_at trigger. Down drops the table and its
// enum types.
func renderMigration(v *view) string {
	type column struct{ name, def string }
	cols := []column{{"id", "BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY"}}
	var constraints, indexes []string
	foreignKey := func(col, table, onDelete string) {
		constraints = append(constraints, fmt.Sprintf("CONSTRAINT %s_%s_fkey FOREIGN KEY (%s)\n        REFERENCES %s (id) ON DELETE %s", v.Table, col, col, table, onDelete))
		indexes = append(indexes, fmt.Sprintf("CREATE INDEX %s_%s_idx ON %s (%s);", v.Table, col, v.Table, col))
	}
	if v.Parent != nil {
		cols = append(cols, column{v.Parent.Column, "BIGINT NOT NULL"})
		foreignKey(v.Parent.Column, v.Parent.Table, "CASCADE")
	} else {
		cols = append(cols, column{"user_id", "BIGINT NOT NULL"})
		foreignKey("user_id", "users", "CASCADE")
	}
	for _, f := range v.Fields {
		cols = append(cols, column{f.Column, f.SQLType})
	}
	for _, r := range v.Refs {
		foreignKey(r.Column, r.Table, "RESTRICT")
	}
	for _, f := range v.Fields {
		if f.Check != "" {
			constraints = append(constraints, fmt.Sprintf("CONSTRAINT %s_%s_check CHECK (%s)", v.Table, f.Column, f.Check))
		}
	}
	cols = append(cols,
		column{"created_at", "TIMESTAMPTZ NOT NULL DEFAULT current_timestamp"},
		column{"updated_at", "TIMESTAMPTZ NOT NULL DEFAULT current_timestamp"})

	width := 0
	for _, c := range cols {
		width = max(width, len(c.name))
	}
	var lines []string
	for _, c := range cols {
		lines = append(lines, fmt.Sprintf("    %-*s  %s", width, c.name, c.def))
	}
	for _, c := range constraints {
		lines = append(lines, "    "+c)
	}

	var b strings.Builder
	b.WriteString("-- +goose Up\n")
	for _, e := range v.Enums {
		values := make([]string, len(e.Consts))
		for i, c := range e.Consts {
			values[i] = "'" + c.Value + "'"
		}
		fmt.Fprintf(&b, "CREATE TYPE %s AS ENUM (%s);\n\n", e.SQLType, strings.Join(values, ", "))
	}
	fmt.Fprintf(&b, "CREATE TABLE %s (\n%s\n);\n\n", v.Table, strings.Join(lines, ",\n"))
	for _, ix := range indexes {
		b.WriteString(ix + "\n")
	}
	fmt.Fprintf(&b, "\nCREATE TRIGGER %s_set_updated_at\n    BEFORE UPDATE ON %s\n    FOR EACH ROW EXECUTE FUNCTION set_updated_at();\n\n", v.Table, v.Table)
	fmt.Fprintf(&b, "-- +goose Down\nDROP TABLE IF EXISTS %s;\n", v.Table)
	for _, e := range v.Enums {
		fmt.Fprintf(&b, "DROP TYPE IF EXISTS %s;\n", e.SQLType)
	}
	return b.String()
}
```

`templates/model.go.tmpl`. The `{{-` markers are placed so the output needs only gofmt:

```gotemplate
package models

import (
	"time"

	"github.com/Oudwins/zog"
	"github.com/uptrace/bun"
)
{{range .Enums}}
// {{.Type}} mirrors the Postgres enum {{.SQLType}}. Declaration order is the enum's sort order.
type {{.Type}} string

const (
{{- $type := .Type}}
{{- range .Consts}}
	{{.Name}} {{$type}} = "{{.Value}}"
{{- end}}
)

// {{.Type}}Values is the allow-list the schema validates against; keep it in step with the
// constants and the CREATE TYPE.
var {{.Type}}Values = []string{ {{- range $i, $c := .Consts}}{{if $i}}, {{end}}string({{$c.Name}}){{end -}} }
{{end}}
type (
	{{.Plural}} = []{{.Name}}
	{{.Name}} struct {
		bun.BaseModel `bun:"table:{{.Table}},alias:{{.Table}}"`

		ID int64 `bun:"id,pk,autoincrement" json:"id"`
{{- if .Parent}}
		{{.Parent.Field}} int64 `bun:"{{.Parent.Column}},notnull" json:"{{.Parent.JSON}}"`
{{- else}}
		UserID int64 `bun:"user_id,notnull" json:"-"`
{{- end}}
{{- range .Fields}}
		{{.GoName}} {{.GoType}} `bun:"{{.BunTag}}" json:"{{.JSON}}"`
{{- end}}

		CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
		UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updatedAt"`
{{if .Parent}}
		{{.Parent.Name}} *{{.Parent.Name}} `bun:"rel:belongs-to,join:{{.Parent.Column}}=id" json:"-"`
{{- else}}
		User *User `bun:"rel:belongs-to,join:user_id=id" json:"-"`
{{- end}}
{{- range .Refs}}
		{{.Relation}} *{{.Model}} `bun:"rel:belongs-to,join:{{.Column}}=id" json:"-"`
{{- end}}
	}
)

// --- Request ---

type {{.Name}}Request struct {
{{- if .Parent}}
	{{.Parent.Field}} int64 `json:"{{.Parent.JSON}}"`
{{- end}}
{{- range .Fields}}
	{{.GoName}} {{.ReqType}} `json:"{{.JSON}}"`
{{- end}}
}

// {{.Name}}Schema validates a {{.Name}}Request. Keys are Go field names; the number schemas
// match the field types exactly.
func {{.Name}}Schema() *zog.StructSchema {
	return zog.Struct(zog.Shape{
{{- if .Parent}}
		"{{.Parent.Field}}": zog.Int64().GT(0).Required(),
{{- end}}
{{- range .Fields}}
		"{{.GoName}}": {{.Schema}},
{{- end}}
	})
}
{{if not .Parent}}
// ToModel is create-only: it stamps the owner, then delegates the fields to ApplyTo.
func (r {{.Name}}Request) ToModel(userID int64) *{{.Name}} {
	{{.Recv}} := &{{.Name}}{UserID: userID}
	r.ApplyTo({{.Recv}})
	return {{.Recv}}
}
{{else if .Movable}}
// ToModel is create-only. ApplyTo copies the parent too, because a {{.LabelLower}} can move.
func (r {{.Name}}Request) ToModel() *{{.Name}} {
	{{.Recv}} := &{{.Name}}{}
	r.ApplyTo({{.Recv}})
	return {{.Recv}}
}
{{else}}
// ToModel is create-only: it stamps the parent, which ApplyTo never copies, so an update
// cannot move the {{.LabelLower}}.
func (r {{.Name}}Request) ToModel() *{{.Name}} {
	{{.Recv}} := &{{.Name}}{ {{- .Parent.Field}}: r.{{.Parent.Field -}} }
	r.ApplyTo({{.Recv}})
	return {{.Recv}}
}
{{end}}
// ApplyTo is the single place client-mutable fields are copied.
func (r {{.Name}}Request) ApplyTo({{.Recv}} *{{.Name}}) {
{{- if .Movable}}
	{{.Recv}}.{{.Parent.Field}} = r.{{.Parent.Field}} // re-parentable: the service verifies the destination
{{- end}}
{{- range .Fields}}
	{{$.Recv}}.{{.GoName}} = {{.Apply}}
{{- end}}
}

// {{.Name}}UpdatableColumns is ApplyTo's field set in column form, for the Update allowlist.
var {{.Name}}UpdatableColumns = []string{ {{- .Updatable -}} }

// --- Response ---

type (
	{{.Name}}Responses = []{{.Name}}Response
	{{.Name}}Response struct {
		ID int64 `json:"id"`
{{- if .Parent}}
		{{.Parent.Field}} int64 `json:"{{.Parent.JSON}}"`
{{- end}}
{{- range .Fields}}
		{{.GoName}} {{.GoType}} `json:"{{.JSON}}"`
{{- end}}
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
)

func New{{.Name}}Response({{.Recv}} *{{.Name}}) {{.Name}}Response {
	return {{.Name}}Response{
		ID: {{.Recv}}.ID,
{{- if .Parent}}
		{{.Parent.Field}}: {{.Recv}}.{{.Parent.Field}},
{{- end}}
{{- range .Fields}}
		{{.GoName}}: {{$.Recv}}.{{.GoName}},
{{- end}}
		CreatedAt: {{.Recv}}.CreatedAt,
		UpdatedAt: {{.Recv}}.UpdatedAt,
	}
}

func New{{.Name}}Responses({{.PluralVar}} {{.Plural}}) {{.Name}}Responses {
	res := make({{.Name}}Responses, len({{.PluralVar}}))
	for i := range {{.PluralVar}} {
		res[i] = New{{.Name}}Response(&{{.PluralVar}}[i])
	}
	return res
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS. If a model test fails, print the rendered source (the `loose` message already includes it) and fix the template's `{{-` markers, not the test.
- [ ] **Step 5:** Commit with the message `resource: render the model and the migration`, plus the trailer.

---

### Task 8: Render the service and the controller

**Files:**
- Create: `internal/generate/resource/templates/service.go.tmpl`, `internal/generate/resource/templates/controller.go.tmpl`
- Modify: `internal/generate/resource/render_test.go`

**Interfaces:**
- Consumes: `view` (Task 6), `renderGo`, `mustRender`, `loose` (Task 7).
- Produces: the templates `service.go.tmpl` and `controller.go.tmpl`.

- [ ] **Step 1: Write the failing tests** (append to `render_test.go`):

```go
func TestRenderServiceFlat(t *testing.T) {
	src := mustRender(t, "service.go.tmpl", mustView(t, "Seminar", []string{"name:string"}, "", false))
	for _, want := range []string{
		"type SeminarsService struct { raptor.Service DB *DatabaseService }",
		"Where(\"seminars.user_id = ?\", userID).",
		"Order(\"seminars.name\", \"seminars.id\").",
		"Where(\"seminars.id = ? AND seminars.user_id = ?\", id, userID).",
		"return seminar, s.DB.HandleErrorNotFound(err, \"Seminar not found\")",
		"func (s *SeminarsService) Create(seminar *models.Seminar) error {",
		"Column(models.SeminarUpdatableColumns...).",
		"Where(\"id = ? AND user_id = ?\", seminar.ID, userID).",
		"func (s *SeminarsService) VerifyOwnership(seminarID, userID int64) error {",
	} {
		loose(t, src, want)
	}
}

func TestRenderServiceChild(t *testing.T) {
	src := mustRender(t, "service.go.tmpl", mustView(t, "Topic", []string{"name:string"}, "Course", false))
	for _, want := range []string{
		"Courses *CoursesService // the immediate parent, injected by type like DB",
		"const topicsOwnedByUser = `topics.course_id IN (SELECT id FROM courses WHERE user_id = ?)`",
		"Order(\"topics.course_id\", \"topics.name\", \"topics.id\").",
		"func (s *TopicsService) ListByCourse(courseID, userID int64) (models.Topics, error) {",
		"Where(\"topics.course_id = ?\", courseID).",
		"func (s *TopicsService) Create(topic *models.Topic, userID int64) error { if err := s.Courses.VerifyOwnership(topic.CourseID, userID); err != nil {",
		"Where(\"topics.id = ?\", topic.ID). Where(topicsOwnedByUser, userID).",
	} {
		loose(t, src, want)
	}
	if n := strings.Count(src, "VerifyOwnership(topic.CourseID"); n != 1 {
		t.Errorf("an immutable parent is verified on create only; found %d checks", n)
	}

	movable := mustRender(t, "service.go.tmpl", mustView(t, "Slot", []string{"name:string"}, "Outcome", true))
	if n := strings.Count(movable, "VerifyOwnership(slot.OutcomeID"); n != 2 {
		t.Errorf("a movable child verifies the destination on update as well; found %d checks", n)
	}
	loose(t, movable, "const slotsOwnedByUser = `slots.outcome_id IN ( SELECT outcomes.id FROM outcomes JOIN courses ON courses.id = outcomes.course_id WHERE courses.user_id = ? )`")
}

func TestRenderControllers(t *testing.T) {
	flat := mustRender(t, "controller.go.tmpl", mustView(t, "Seminar", []string{"name:string"}, "", false))
	for _, want := range []string{
		"type SeminarsController struct { raptor.Controller Auth *services.AuthService Seminars *services.SeminarsService Validation *services.ValidationService }",
		"seminars, err := c.Seminars.List(user.ID)",
		"if issues := c.Validation.SeminarSchema.Validate(&req); len(issues) > 0 {",
		"seminar := req.ToModel(user.ID)",
		"if err := c.Seminars.Create(seminar); err != nil {",
		"return ctx.Data(models.NewSeminarResponse(seminar), http.StatusCreated)",
		"seminar := &models.Seminar{ID: id} // identity from the path, state from the body",
		"return ctx.NoContent()",
	} {
		loose(t, flat, want)
	}

	child := mustRender(t, "controller.go.tmpl", mustView(t, "Topic", []string{"name:string"}, "Course", false))
	for _, want := range []string{
		"if raw := ctx.QueryParam(\"courseId\"); raw == \"\" {",
		"return errs.NewErrorBadRequest(\"Invalid courseId\")",
		"topics, err = c.Topics.ListByCourse(courseID, user.ID)",
		"topic := req.ToModel()",
		"if err := c.Topics.Create(topic, user.ID); err != nil {",
	} {
		loose(t, child, want)
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run 'RenderService|RenderControllers'`. Expected: FAIL (`no template "service.go.tmpl"`).
- [ ] **Step 3: Implement.** `templates/service.go.tmpl`:

```gotemplate
package services

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
	"{{.Module}}/app/models"
)

type {{.Plural}}Service struct {
	raptor.Service

	DB *DatabaseService
{{- if .Parent}}
	{{.Parent.Plural}} *{{.Parent.Plural}}Service // the immediate parent, injected by type like DB
{{- end}}
}
{{if .Parent}}
// {{.PredicateConst}} scopes a {{.LabelLower}} to its owner through its parent. Get, Update,
// Delete and VerifyOwnership share it, so the ownership rule is written once.
const {{.PredicateConst}} = `{{.Predicate}}`

// List is every {{.LabelLower}} the user owns: the unfiltered index.
func (s *{{.Plural}}Service) List(userID int64) (models.{{.Plural}}, error) {
	var {{.PluralVar}} models.{{.Plural}}
	err := s.DB.Conn().NewSelect().
		Model(&{{.PluralVar}}).
		Where({{.PredicateConst}}, userID).
		Order({{.ChildOrder}}).
		Scan(s.DB.Ctx)
	return {{.PluralVar}}, s.DB.HandleError(err)
}

// ListBy{{.Parent.Name}} verifies the parent first: filtering alone could not tell "not your
// {{.Parent.LabelLower}}" from "a {{.Parent.LabelLower}} with no {{.PluralLabelLower}}", which are both zero rows.
func (s *{{.Plural}}Service) ListBy{{.Parent.Name}}({{.Parent.Var}}ID, userID int64) (models.{{.Plural}}, error) {
	if err := s.{{.Parent.Plural}}.VerifyOwnership({{.Parent.Var}}ID, userID); err != nil {
		return nil, err
	}
	var {{.PluralVar}} models.{{.Plural}}
	err := s.DB.Conn().NewSelect().
		Model(&{{.PluralVar}}).
		Where("{{.Table}}.{{.Parent.Column}} = ?", {{.Parent.Var}}ID).
		Order({{.OrderBy}}).
		Scan(s.DB.Ctx)
	return {{.PluralVar}}, s.DB.HandleError(err)
}

func (s *{{.Plural}}Service) Get(id, userID int64) (*models.{{.Name}}, error) {
	{{.Var}} := new(models.{{.Name}})
	err := s.DB.Conn().NewSelect().
		Model({{.Var}}).
		Where("{{.Table}}.id = ?", id).
		Where({{.PredicateConst}}, userID).
		Scan(s.DB.Ctx)
	return {{.Var}}, s.DB.HandleErrorNotFound(err, "{{.Label}} not found")
}

func (s *{{.Plural}}Service) Create({{.Var}} *models.{{.Name}}, userID int64) error {
	if err := s.{{.Parent.Plural}}.VerifyOwnership({{.Var}}.{{.Parent.Field}}, userID); err != nil {
		return err
	}
	_, err := s.DB.Conn().NewInsert().
		Model({{.Var}}).
		Returning("*").
		Exec(s.DB.Ctx)
	return s.DB.HandleError(err)
}

func (s *{{.Plural}}Service) Update({{.Var}} *models.{{.Name}}, userID int64) error {
{{- if .Movable}}
	// The WHERE below sees pre-update values, so it only proves the *current* {{.Parent.LabelLower}}
	// is the user's. The destination needs its own check.
	if err := s.{{.Parent.Plural}}.VerifyOwnership({{.Var}}.{{.Parent.Field}}, userID); err != nil {
		return err
	}
{{- end}}
	res, err := s.DB.Conn().NewUpdate().
		Model({{.Var}}).
		Column(models.{{.Name}}UpdatableColumns...).
		Where("{{.Table}}.id = ?", {{.Var}}.ID).
		Where({{.PredicateConst}}, userID).
		Returning("*").
		Exec(s.DB.Ctx)
	return s.DB.HandleAffected(res, err, "{{.Label}} not found")
}

func (s *{{.Plural}}Service) Delete(id, userID int64) error {
	res, err := s.DB.Conn().NewDelete().
		Model((*models.{{.Name}})(nil)).
		Where("{{.Table}}.id = ?", id).
		Where({{.PredicateConst}}, userID).
		Exec(s.DB.Ctx)
	return s.DB.HandleAffected(res, err, "{{.Label}} not found")
}

// VerifyOwnership is what child services call before creating or listing under a
// {{.LabelLower}}. It uses the same predicate as Get.
func (s *{{.Plural}}Service) VerifyOwnership({{.Var}}ID, userID int64) error {
	exists, err := s.DB.Conn().NewSelect().
		Model((*models.{{.Name}})(nil)).
		Where("{{.Table}}.id = ?", {{.Var}}ID).
		Where({{.PredicateConst}}, userID).
		Exists(s.DB.Ctx)
	if err != nil {
		return s.DB.HandleError(err)
	}
	if !exists {
		return errs.NewErrorNotFound("{{.Label}} not found")
	}
	return nil
}
{{- else}}
func (s *{{.Plural}}Service) List(userID int64) (models.{{.Plural}}, error) {
	var {{.PluralVar}} models.{{.Plural}}
	err := s.DB.Conn().NewSelect().
		Model(&{{.PluralVar}}).
		Where("{{.Table}}.user_id = ?", userID).
		Order({{.OrderBy}}).
		Scan(s.DB.Ctx)
	return {{.PluralVar}}, s.DB.HandleError(err)
}

func (s *{{.Plural}}Service) Get(id, userID int64) (*models.{{.Name}}, error) {
	{{.Var}} := new(models.{{.Name}})
	err := s.DB.Conn().NewSelect().
		Model({{.Var}}).
		Where("{{.Table}}.id = ? AND {{.Table}}.user_id = ?", id, userID).
		Scan(s.DB.Ctx)
	return {{.Var}}, s.DB.HandleErrorNotFound(err, "{{.Label}} not found")
}

func (s *{{.Plural}}Service) Create({{.Var}} *models.{{.Name}}) error {
	_, err := s.DB.Conn().NewInsert().
		Model({{.Var}}).
		Returning("*").
		Exec(s.DB.Ctx)
	return s.DB.HandleError(err)
}

// Update writes only the allowlisted columns; the WHERE scopes the write to the owner, and
// Returning("*") brings back the trigger-set updated_at.
func (s *{{.Plural}}Service) Update({{.Var}} *models.{{.Name}}, userID int64) error {
	res, err := s.DB.Conn().NewUpdate().
		Model({{.Var}}).
		Column(models.{{.Name}}UpdatableColumns...).
		Where("id = ? AND user_id = ?", {{.Var}}.ID, userID).
		Returning("*").
		Exec(s.DB.Ctx)
	return s.DB.HandleAffected(res, err, "{{.Label}} not found")
}

func (s *{{.Plural}}Service) Delete(id, userID int64) error {
	res, err := s.DB.Conn().NewDelete().
		Model((*models.{{.Name}})(nil)).
		Where("id = ? AND user_id = ?", id, userID).
		Exec(s.DB.Ctx)
	return s.DB.HandleAffected(res, err, "{{.Label}} not found")
}

// VerifyOwnership is what child services call before creating or listing under a
// {{.LabelLower}}. Exists: confirmation is all that's needed, not the row.
func (s *{{.Plural}}Service) VerifyOwnership({{.Var}}ID, userID int64) error {
	exists, err := s.DB.Conn().NewSelect().
		Model((*models.{{.Name}})(nil)).
		Where("id = ? AND user_id = ?", {{.Var}}ID, userID).
		Exists(s.DB.Ctx)
	if err != nil {
		return s.DB.HandleError(err)
	}
	if !exists {
		return errs.NewErrorNotFound("{{.Label}} not found")
	}
	return nil
}
{{- end}}
```

`templates/controller.go.tmpl`:

```gotemplate
package controllers

import (
	"net/http"
{{- if .Parent}}
	"strconv"
{{- end}}

	"github.com/go-raptor/raptor/v4"
{{- if .Parent}}
	"github.com/go-raptor/raptor/v4/errs"
{{- end}}
	"{{.Module}}/app/models"
	"{{.Module}}/app/services"
)

type {{.Plural}}Controller struct {
	raptor.Controller

	Auth *services.AuthService
	{{.Plural}} *services.{{.Plural}}Service
	Validation *services.ValidationService
}
{{if .Parent}}
// Index lists every {{.LabelLower}} the user owns, or one {{.Parent.LabelLower}}'s when ?{{.Parent.JSON}}= is given.
func (c *{{.Plural}}Controller) Index(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}

	var {{.PluralVar}} models.{{.Plural}}
	if raw := ctx.QueryParam("{{.Parent.JSON}}"); raw == "" {
		{{.PluralVar}}, err = c.{{.Plural}}.List(user.ID)
	} else {
		{{.Parent.Var}}ID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			return errs.NewErrorBadRequest("Invalid {{.Parent.JSON}}")
		}
		{{.PluralVar}}, err = c.{{.Plural}}.ListBy{{.Parent.Name}}({{.Parent.Var}}ID, user.ID) // 404 if the {{.Parent.LabelLower}} isn't theirs
	}
	if err != nil {
		return err
	}
	return ctx.Data(models.New{{.Name}}Responses({{.PluralVar}}))
}
{{- else}}
func (c *{{.Plural}}Controller) Index(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}
	{{.PluralVar}}, err := c.{{.Plural}}.List(user.ID)
	if err != nil {
		return err
	}
	return ctx.Data(models.New{{.Name}}Responses({{.PluralVar}}))
}
{{- end}}

func (c *{{.Plural}}Controller) Show(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	{{.Var}}, err := c.{{.Plural}}.Get(id, user.ID)
	if err != nil {
		return err
	}
	return ctx.Data(models.New{{.Name}}Response({{.Var}}))
}

func (c *{{.Plural}}Controller) Create(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}
	var req models.{{.Name}}Request
	if err := bindJSON(ctx, &req); err != nil {
		return err
	}
	if issues := c.Validation.{{.Name}}Schema.Validate(&req); len(issues) > 0 {
		return validationFailed(issues)
	}
{{- if .Parent}}
	{{.Var}} := req.ToModel()
	if err := c.{{.Plural}}.Create({{.Var}}, user.ID); err != nil { // verifies the {{.Parent.LabelLower}}
		return err
	}
{{- else}}
	{{.Var}} := req.ToModel(user.ID)
	if err := c.{{.Plural}}.Create({{.Var}}); err != nil {
		return err
	}
{{- end}}
	return ctx.Data(models.New{{.Name}}Response({{.Var}}), http.StatusCreated)
}

func (c *{{.Plural}}Controller) Update(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	var req models.{{.Name}}Request
	if err := bindJSON(ctx, &req); err != nil {
		return err
	}
	if issues := c.Validation.{{.Name}}Schema.Validate(&req); len(issues) > 0 {
		return validationFailed(issues)
	}
	{{.Var}} := &models.{{.Name}}{ID: id} // identity from the path, state from the body
	req.ApplyTo({{.Var}})
	if err := c.{{.Plural}}.Update({{.Var}}, user.ID); err != nil {
		return err
	}
	return ctx.Data(models.New{{.Name}}Response({{.Var}}))
}

func (c *{{.Plural}}Controller) Destroy(ctx *raptor.Context) error {
	user, err := c.Auth.CurrentUser(ctx)
	if err != nil {
		return err
	}
	id, err := pathID(ctx)
	if err != nil {
		return err
	}
	if err := c.{{.Plural}}.Delete(id, user.ID); err != nil {
		return err
	}
	return ctx.NoContent()
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: render the service and the controller`, plus the trailer.

---

### Task 9: Render the shared files the generator bootstraps

These are `patterns.md` and `testing.md`, verbatim except for the module path and, in `validation_service.go`, the one schema. `newUser` gains a `t.Cleanup` that deletes the user; that deletion cascades through the user's rows.

**Files:**
- Create in `internal/generate/resource/templates/`: `database_service.go.tmpl`, `validation_service.go.tmpl`, `helpers.go.tmpl`, `validation.go.tmpl`, `schemas_test.go.tmpl`, `setup_test.go.tmpl`, `harness_test.go.tmpl`
- Modify: `internal/generate/resource/render_test.go`

**Interfaces:**
- Consumes: `view` (`.Module` and `.Name`), `renderGo`.
- Produces: the seven templates above.

- [ ] **Step 1: Write the failing test** (append to `render_test.go`):

```go
func TestRenderBootstrapFiles(t *testing.T) {
	v := mustView(t, "Seminar", []string{"name:string"}, "", false)
	for tmpl, wants := range map[string][]string{
		"database_service.go.tmpl": {
			"func (s *DatabaseService) Conn() *bun.DB {",
			"func (s *DatabaseService) HandleAffected(res sql.Result, err error, notFound string) error {",
			"case \"23505\": // unique_violation",
		},
		"validation_service.go.tmpl": {
			"\"example.com/shop/app/models\"",
			"SeminarSchema *zog.StructSchema",
			"s.SeminarSchema = models.SeminarSchema() return nil",
		},
		"helpers.go.tmpl": {
			"func bindJSON(ctx *raptor.Context, v any) error {",
			"func validationFailed(issues zog.ZogIssueList) error {",
			"func pathID(ctx *raptor.Context) (int64, error) {",
		},
		"validation.go.tmpl": {"func maxChars(n int) (zog.BoolTFunc[*string], zog.TestOption) {"},
		"schemas_test.go.tmpl": {
			"package models_test",
			"func TestSchemasMatchTheirRequests(t *testing.T) { models.SeminarSchema().Validate(&models.SeminarRequest{}) }",
		},
		"setup_test.go.tmpl": {
			"\"example.com/shop/config/components\"",
			"os.Setenv(\"SPA_OPTIONAL\", \"true\")",
			"app = raptor.NewTestApp(components.New(), config.Routes())",
		},
		"harness_test.go.tmpl": {
			"func db(t *testing.T) *bun.DB {",
			"func mustInsert(t *testing.T, q *bun.InsertQuery) {",
			"func newUser(t *testing.T, username string) *models.User {",
			"t.Cleanup(func() { _, _ = db(t).NewDelete().Model((*models.User)(nil)).Where(\"id = ?\", user.ID).Exec(context.Background()) })",
			"return raptor.WithRemoteAddr(fmt.Sprintf(\"10.%d.%d.%d\", byte(n>>16), byte(n>>8), byte(n)))",
			"func login(t *testing.T, username string) *http.Cookie {",
			"func withSession(c *http.Cookie) raptor.TestRequestOption {",
		},
	} {
		src := mustRender(t, tmpl, v)
		for _, want := range wants {
			loose(t, src, want)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run Bootstrap`. Expected: FAIL (`no template "database_service.go.tmpl"`).
- [ ] **Step 3: Implement.** `templates/database_service.go.tmpl`:

```gotemplate
package services

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/uptrace/bun"
)

type DatabaseService struct {
	raptor.Service

	// Ctx is the context CRUD queries run on. It is background on purpose: work that should stop
	// when the client disconnects takes a context.Context parameter instead.
	Ctx context.Context
}

func (s *DatabaseService) Setup() error {
	s.Ctx = context.Background()
	return nil
}

func (s *DatabaseService) Conn() *bun.DB {
	return s.Database.Conn().(*bun.DB)
}

// HandleError maps a query error onto an errs value. A missing row is not an error at this
// level: lists come back empty, writes check RowsAffected, and single reads use
// HandleErrorNotFound.
func (s *DatabaseService) HandleError(err error) error {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return s.handlePostgresError(pgErr)
	}
	s.Log.Error("Unhandled database error", "error", err)
	return errs.NewErrorInternal("Database error")
}

// HandleErrorNotFound is HandleError for single-row reads, where no row means 404.
func (s *DatabaseService) HandleErrorNotFound(err error, message ...string) error {
	if errors.Is(err, sql.ErrNoRows) {
		msg := "Resource not found"
		if len(message) > 0 {
			msg = message[0]
		}
		return errs.NewErrorNotFound(msg)
	}
	return s.HandleError(err)
}

// HandleAffected finishes an Update or Delete: it maps a query error, and turns a write that
// matched no row into a 404. Neither statement reports sql.ErrNoRows, so without this check a
// write aimed at someone else's row would answer 200.
func (s *DatabaseService) HandleAffected(res sql.Result, err error, notFound string) error {
	if err != nil {
		return s.HandleError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return s.HandleError(err)
	}
	if n == 0 {
		return errs.NewErrorNotFound(notFound)
	}
	return nil
}

// handlePostgresError gives the client a status and at most the constraint name. The driver's
// message and detail echo submitted values and the schema, so they go to the log only.
func (s *DatabaseService) handlePostgresError(pgErr *pgconn.PgError) error {
	var attrs []any
	if pgErr.ConstraintName != "" {
		attrs = []any{"constraint", pgErr.ConstraintName}
	}

	var mapped *errs.Error
	switch pgErr.Code {
	case "23505": // unique_violation
		mapped = errs.NewErrorConflict("Unique constraint violation", attrs...)
	case "23503": // foreign_key_violation
		mapped = errs.NewErrorConflict("Foreign key violation", attrs...)
	case "23502": // not_null_violation
		mapped = errs.NewErrorUnprocessableEntity("Missing required value", attrs...)
	case "23514": // check_violation
		mapped = errs.NewErrorUnprocessableEntity("Check constraint violation", attrs...)
	case "22001": // string_data_right_truncation
		mapped = errs.NewErrorUnprocessableEntity("Value too long", attrs...)
	case "22003": // numeric_value_out_of_range
		mapped = errs.NewErrorUnprocessableEntity("Number out of range", attrs...)
	case "22P02": // invalid_text_representation, e.g. a value outside a Postgres enum
		mapped = errs.NewErrorUnprocessableEntity("Invalid value", attrs...)
	default:
		s.Log.Error("Unhandled database error", "code", pgErr.Code, "message", pgErr.Message)
		return errs.NewErrorInternal("Database error")
	}

	s.Log.Warn("Database constraint error",
		"code", pgErr.Code, "constraint", pgErr.ConstraintName,
		"message", pgErr.Message, "detail", pgErr.Detail)
	return mapped
}
```

`templates/validation_service.go.tmpl`:

```gotemplate
package services

import (
	"github.com/Oudwins/zog"
	"github.com/go-raptor/raptor/v4"
	"{{.Module}}/app/models"
)

// ValidationService builds every schema once, at startup, and shares it with every request.
type ValidationService struct {
	raptor.Service

	{{.Name}}Schema *zog.StructSchema
}

func (s *ValidationService) Setup() error {
	s.{{.Name}}Schema = models.{{.Name}}Schema()
	return nil
}
```

`templates/helpers.go.tmpl`:

```gotemplate
package controllers

import (
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/Oudwins/zog"
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
)

// bindJSON decodes a JSON request body.
//   - The body must be declared application/json, or the answer is 415. That check is what makes
//     "JSON-only bodies" a real CSRF defense: ctx.Bind ignores Content-Type, and a cross-site
//     form can post text/plain whose body happens to be valid JSON.
//   - Malformed JSON is a 400. An oversized body keeps its *http.MaxBytesError, which Raptor
//     answers with 413.
func bindJSON(ctx *raptor.Context, v any) error {
	mediaType, _, err := mime.ParseMediaType(ctx.Request().Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errs.NewErrorUnsupportedMediaType("Expected an application/json body")
	}
	if err := ctx.Bind(v); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return err
		}
		return errs.NewErrorBadRequest("Invalid JSON body")
	}
	return nil
}

// validationFailed is the 422 for a schema failure. Flatten keeps only field names and
// messages: the raw issues carry the submitted values, which must not reach the client or the
// request log.
func validationFailed(issues zog.ZogIssueList) error {
	return errs.NewErrorUnprocessableEntity("Validation failed", "issues", zog.Issues.Flatten(issues))
}

// pathID parses the {id} path parameter; garbage is a 400.
func pathID(ctx *raptor.Context) (int64, error) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		return 0, errs.NewErrorBadRequest("Invalid ID")
	}
	return id, nil
}
```

`templates/validation.go.tmpl`:

```gotemplate
package models

import (
	"fmt"
	"unicode/utf8"

	"github.com/Oudwins/zog"
)

// maxChars limits a string by characters. zog's Max counts bytes, and č, ć, š, ž and đ take two
// bytes each, while VARCHAR(n) and the frontend's zod .max(n) count characters. Use it as
// zog.String().TestFunc(maxChars(150)).
func maxChars(n int) (zog.BoolTFunc[*string], zog.TestOption) {
	return func(s *string, _ zog.Ctx) bool {
			return utf8.RuneCountInString(*s) <= n
		},
		zog.Message(fmt.Sprintf("must be at most %d characters", n))
}
```

`templates/schemas_test.go.tmpl`:

```gotemplate
package models_test

import (
	"testing"

	"{{.Module}}/app/models"
)

// A number schema whose type doesn't match its field panics inside Validate. The zero-value
// request reaches every field, so this one test catches every mismatch at go test time.
func TestSchemasMatchTheirRequests(t *testing.T) {
	models.{{.Name}}Schema().Validate(&models.{{.Name}}Request{})
}
```

`templates/setup_test.go.tmpl`:

```gotemplate
package controllers_test

import (
	"os"
	"testing"

	"github.com/go-raptor/raptor/v4"
	"{{.Module}}/config"
	"{{.Module}}/config/components"
)

var app *raptor.Raptor

func TestMain(m *testing.M) {
	// spa/v2 refuses to boot without a frontend build unless it is Optional, and components.New
	// reads the flag, so set it first. Apps without the SPA controller ignore it.
	os.Setenv("SPA_OPTIONAL", "true")
	app = raptor.NewTestApp(components.New(), config.Routes())
	os.Exit(m.Run())
}
```

`templates/harness_test.go.tmpl`:

```gotemplate
package controllers_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/go-raptor/raptor/v4"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
	"{{.Module}}/app/models"
	"{{.Module}}/app/services"
)

const testPassword = "test-password"

func db(t *testing.T) *bun.DB {
	t.Helper()
	service := raptor.GetService[services.DatabaseService](app)
	if service == nil {
		t.Fatal("DatabaseService is not registered")
	}
	return service.Conn()
}

func mustInsert(t *testing.T, q *bun.InsertQuery) {
	t.Helper()
	if _, err := q.Exec(t.Context()); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
}

// newUser inserts a user who can log in with testPassword. Deleting the user when the test
// ends cascades through everything the user owns.
func newUser(t *testing.T, username string) *models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost) // MinCost: fast tests
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{Username: username, Password: string(hash), Email: username + "@example.test"}
	mustInsert(t, db(t).NewInsert().Model(user).Returning("*"))
	t.Cleanup(func() {
		_, _ = db(t).NewDelete().Model((*models.User)(nil)).Where("id = ?", user.ID).Exec(context.Background())
	})
	return user
}

var clientIPs atomic.Uint32

// newClient simulates a separate browser, with its own address and so its own limiter bucket.
func newClient() raptor.TestRequestOption {
	n := clientIPs.Add(1)
	return raptor.WithRemoteAddr(fmt.Sprintf("10.%d.%d.%d", byte(n>>16), byte(n>>8), byte(n)))
}

func login(t *testing.T, username string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": testPassword})
	rec := app.TestPost("/api/v1/auth/login", bytes.NewReader(body), newClient())
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("login set no session cookie")
	return nil
}

func withSession(c *http.Cookie) raptor.TestRequestOption {
	return raptor.WithHeader("Cookie", c.Name+"="+c.Value)
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: render the shared services, helpers and test harness`, plus the trailer.

---

### Task 10: Render the seeds and the integration tests

**Files:**
- Create: `internal/generate/resource/seed.go`, `internal/generate/resource/seed_test.go`
- Create in `internal/generate/resource/templates/`: `seed.go.tmpl`, `seed_model.go.tmpl`, `controller_test.go.tmpl`
- Modify: `internal/generate/resource/render_test.go`

**Interfaces:**
- Consumes: `view`, `ModelIndex`, `IsOwned`, `FKTarget`, `renderGo`.
- Produces:

```go
type seedModelView struct {
	Module, Name, Var    string
	Owned                bool // the seed takes userID
	Fields               []seedFieldView
	UsesTime, UsesSuffix bool
}
type seedFieldView struct{ GoName, Expr string }
func buildSeedModel(idx ModelIndex, m *Model, module string) (*seedModelView, error)
```

- [ ] **Step 1: Write the failing tests.** `seed_test.go`:

```go
package resource

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSeedModel(t *testing.T) {
	idx := loadFixtureModels(t)
	tests := []struct {
		model  string
		owned  bool
		fields []seedFieldView
	}{
		{"Division", false, []seedFieldView{{"Name", `"Name " + suffix`}}},
		{"Course", true, []seedFieldView{{"UserID", "userID"}, {"DivisionID", "seedDivision(t).ID"}, {"Name", `"Name " + suffix`}}},
		{"Outcome", true, []seedFieldView{{"CourseID", "seedCourse(t, userID).ID"}, {"Name", `"Name " + suffix`}}},
	}
	for _, tt := range tests {
		s, err := buildSeedModel(idx, idx[tt.model], "example.com/shop")
		if err != nil {
			t.Fatalf("%s: %v", tt.model, err)
		}
		if s.Owned != tt.owned || !reflect.DeepEqual(s.Fields, tt.fields) {
			t.Errorf("%s: owned=%v fields=%+v", tt.model, s.Owned, s.Fields)
		}
	}
	if _, err := buildSeedModel(idx, idx["Unit"], "example.com/shop"); err == nil || !strings.Contains(err.Error(), "no sample value for type UnitType") {
		t.Errorf("an unknown field type must be reported: %v", err)
	}
}
```

Append to `render_test.go`:

```go
func TestRenderSeeds(t *testing.T) {
	flat := mustRender(t, "seed.go.tmpl", mustView(t, "Seminar", []string{"name:string", "lecture_hours:int", "division:ref", "notes:text"}, "", false))
	for _, want := range []string{
		"func seedSeminar(t *testing.T, userID int64) *models.Seminar {",
		"seminar := &models.Seminar{ UserID: userID, Name: \"Sample\", LectureHours: 1, DivisionID: seedDivision(t).ID, Notes: \"Sample text\", }",
		"mustInsert(t, db(t).NewInsert().Model(seminar).Returning(\"*\"))",
	} {
		loose(t, flat, want)
	}

	child := mustRender(t, "seed.go.tmpl", mustView(t, "Slot", []string{"kind:enum:lecture,lab", "starts_at:time"}, "Outcome", true))
	for _, want := range []string{
		"\"time\"",
		"OutcomeID: seedOutcome(t, userID).ID,",
		"Kind: models.SlotKindLecture,",
		"StartsAt: time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC),",
	} {
		loose(t, child, want)
	}

	idx := loadFixtureModels(t)
	division, err := buildSeedModel(idx, idx["Division"], "example.com/shop")
	if err != nil {
		t.Fatal(err)
	}
	src := mustRender(t, "seed_model.go.tmpl", division)
	for _, want := range []string{
		"func seedDivision(t *testing.T) *models.Division {",
		"suffix := strconv.FormatInt(time.Now().UnixNano(), 36)",
		"division := &models.Division{ Name: \"Name \" + suffix, }",
	} {
		loose(t, src, want)
	}
}

func TestRenderControllerTests(t *testing.T) {
	flat := mustRender(t, "controller_test.go.tmpl", mustView(t, "Seminar", []string{"name:string", "division:ref"}, "", false))
	for _, want := range []string{
		"func seminarsRequest(t *testing.T, method, path string, session *http.Cookie, body any) *httptest.ResponseRecorder {",
		"func validSeminarRequest(t *testing.T, userID int64) models.SeminarRequest { t.Helper() return models.SeminarRequest{ Name: \"Sample\", DivisionID: seedDivision(t).ID, } }",
		"func TestSeminarsRequireAuth(t *testing.T) {",
		"rec := seminarsRequest(t, http.MethodPost, \"/api/v1/seminars\", session, req)",
		"req.Name = \"Updated\"",
		"if updated.Name != \"Updated\" {",
		"func TestSeminarsStrangerGets404(t *testing.T) {",
		"seminar := seedSeminar(t, owner.ID)",
		"func TestSeminarsCreateValidation(t *testing.T) {",
	} {
		loose(t, flat, want)
	}
	if strings.Contains(flat, "UnderStrangers") {
		t.Error("a flat resource has no parent test")
	}

	child := mustRender(t, "controller_test.go.tmpl", mustView(t, "Topic", []string{"name:string"}, "Course", false))
	for _, want := range []string{
		"CourseID: seedCourse(t, userID).ID,",
		"func TestTopicsUnderStrangersCourse(t *testing.T) {",
		"req.CourseID = strangersCourse.ID",
		"list := fmt.Sprintf(\"/api/v1/topics?courseId=%d\", strangersCourse.ID)",
	} {
		loose(t, child, want)
	}

	optional := mustRender(t, "controller_test.go.tmpl", mustView(t, "Counter", []string{"value:int"}, "", false))
	if strings.Contains(optional, "CreateValidation") || strings.Contains(optional, "Updated") {
		t.Error("with no required field, there is no validation test and no update assertion")
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run 'Seed|ControllerTests'`. Expected: FAIL (undefined `buildSeedModel`).
- [ ] **Step 3: Implement.** `seed.go`:

```go
package resource

import (
	"fmt"
	"strings"

	"github.com/go-raptor/cli/internal/naming"
)

// seedModelView renders a seed function for a model the generator didn't write: a parent or a
// ref target that a generated test needs rows of.
type seedModelView struct {
	Module     string
	Name, Var  string
	Owned      bool // the seed takes userID
	Fields     []seedFieldView
	UsesTime   bool
	UsesSuffix bool
}

type seedFieldView struct{ GoName, Expr string }

var numericTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true,
}

// buildSeedModel picks a sample value for every column: userID for the owner, another seed for a
// foreign key, and a literal by Go type otherwise. Pointer fields stay nil. A type it has no
// sample for is an error, which skips the integration tests rather than guessing.
func buildSeedModel(idx ModelIndex, m *Model, module string) (*seedModelView, error) {
	s := &seedModelView{Module: module, Name: m.Name, Var: naming.Var(m.Name), Owned: idx.IsOwned(m.Name)}
	for _, f := range m.Fields {
		switch f.Column {
		case "", "id", "created_at", "updated_at":
			continue
		case "user_id":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "userID"})
			continue
		}
		if strings.HasPrefix(f.GoType, "*") {
			continue
		}
		if target, ok := idx.FKTarget(m, f.Column); ok {
			expr := "seed" + target.Name + "(t).ID"
			if idx.IsOwned(target.Name) {
				expr = "seed" + target.Name + "(t, userID).ID"
			}
			s.Fields = append(s.Fields, seedFieldView{f.GoName, expr})
			continue
		}
		switch {
		case f.GoType == "string":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, fmt.Sprintf("%q + suffix", f.GoName+" ")})
			s.UsesSuffix = true
		case numericTypes[f.GoType]:
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "1"})
		case f.GoType == "bool":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "true"})
		case f.GoType == "time.Time":
			s.Fields = append(s.Fields, seedFieldView{f.GoName, "time.Now()"})
			s.UsesTime = true
		default:
			return nil, fmt.Errorf("cannot seed %s.%s: the generator has no sample value for type %s", m.Name, f.GoName, f.GoType)
		}
	}
	return s, nil
}
```

`templates/seed.go.tmpl`:

```gotemplate
package controllers_test

import (
	"context"
	"testing"
{{- if .SamplesUseTime}}
	"time"
{{- end}}

	"{{.Module}}/app/models"
)

// seed{{.Name}} inserts a {{.LabelLower}} owned by userID, with sample values, and deletes it
// when the test ends.
func seed{{.Name}}(t *testing.T, userID int64) *models.{{.Name}} {
	t.Helper()
	{{.Var}} := &models.{{.Name}}{
{{- if .Parent}}
		{{.Parent.Field}}: seed{{.Parent.Name}}(t, userID).ID,
{{- else}}
		UserID: userID,
{{- end}}
{{- range .Fields}}{{if .SampleModel}}
		{{.GoName}}: {{.SampleModel}},
{{- end}}{{end}}
	}
	mustInsert(t, db(t).NewInsert().Model({{.Var}}).Returning("*"))
	t.Cleanup(func() {
		_, _ = db(t).NewDelete().Model((*models.{{.Name}})(nil)).Where("id = ?", {{.Var}}.ID).Exec(context.Background())
	})
	return {{.Var}}
}
```

`templates/seed_model.go.tmpl`:

```gotemplate
package controllers_test

import (
	"context"
{{- if .UsesSuffix}}
	"strconv"
{{- end}}
	"testing"
{{- if or .UsesTime .UsesSuffix}}
	"time"
{{- end}}

	"{{.Module}}/app/models"
)

// seed{{.Name}} inserts a {{.Name}} with sample values and deletes it when the test ends.
func seed{{.Name}}(t *testing.T{{if .Owned}}, userID int64{{end}}) *models.{{.Name}} {
	t.Helper()
{{- if .UsesSuffix}}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
{{- end}}
	{{.Var}} := &models.{{.Name}}{
{{- range .Fields}}
		{{.GoName}}: {{.Expr}},
{{- end}}
	}
	mustInsert(t, db(t).NewInsert().Model({{.Var}}).Returning("*"))
	t.Cleanup(func() {
		_, _ = db(t).NewDelete().Model((*models.{{.Name}})(nil)).Where("id = ?", {{.Var}}.ID).Exec(context.Background())
	})
	return {{.Var}}
}
```

`templates/controller_test.go.tmpl`:

```gotemplate
package controllers_test

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-raptor/raptor/v4"
	"{{.Module}}/app/models"
)

// {{.PluralVar}}Request sends body as JSON, with the session cookie when there is one.
func {{.PluralVar}}Request(t *testing.T, method, path string, session *http.Cookie, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	var opts []raptor.TestRequestOption
	if session != nil {
		opts = append(opts, withSession(session))
	}
	return app.TestRequest(method, path, reader, opts...)
}

// valid{{.Name}}Request is a request that passes validation{{if .Parent}}, under a {{.Parent.LabelLower}} owned by userID{{end}}.
func valid{{.Name}}Request(t *testing.T, userID int64) models.{{.Name}}Request {
	t.Helper()
	return models.{{.Name}}Request{
{{- if .Parent}}
		{{.Parent.Field}}: seed{{.Parent.Name}}(t, userID).ID,
{{- end}}
{{- range .Fields}}{{if .SampleReq}}
		{{.GoName}}: {{.SampleReq}},
{{- end}}{{end}}
	}
}

func Test{{.Plural}}RequireAuth(t *testing.T) {
	if rec := {{.PluralVar}}Request(t, http.MethodGet, "/api/v1/{{.Route}}", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET without a session: %d, want 401", rec.Code)
	}
}

func Test{{.Plural}}OwnerLifecycle(t *testing.T) {
	owner := newUser(t, fmt.Sprintf("owner-%d", time.Now().UnixNano()))
	session := login(t, owner.Username)
	req := valid{{.Name}}Request(t, owner.ID)

	rec := {{.PluralVar}}Request(t, http.MethodPost, "/api/v1/{{.Route}}", session, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created models.{{.Name}}Response
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.CreatedAt.IsZero() {
		t.Fatalf("create must return the stored row: %+v", created)
	}
	path := fmt.Sprintf("/api/v1/{{.Route}}/%d", created.ID)

	if rec := {{.PluralVar}}Request(t, http.MethodGet, path, session, nil); rec.Code != http.StatusOK {
		t.Fatalf("show: %d %s", rec.Code, rec.Body)
	}
{{- with .UpdatedField}}

	req.{{.GoName}} = {{.UpdateSample}}
{{- end}}
	rec = {{.PluralVar}}Request(t, http.MethodPut, path, session, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
{{- with .UpdatedField}}
	var updated models.{{$.Name}}Response
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.{{.GoName}} != {{.UpdateSample}} {
		t.Fatalf("the update did not persist: %+v", updated)
	}
{{- end}}

	rec = {{.PluralVar}}Request(t, http.MethodGet, "/api/v1/{{.Route}}", session, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var listed models.{{.Name}}Responses
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(listed, func(x models.{{.Name}}Response) bool { return x.ID == created.ID }) {
		t.Fatalf("the list does not include the new {{.LabelLower}}: %+v", listed)
	}

	if rec := {{.PluralVar}}Request(t, http.MethodDelete, path, session, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := {{.PluralVar}}Request(t, http.MethodGet, path, session, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("show after delete: %d, want 404", rec.Code)
	}
}

func Test{{.Plural}}StrangerGets404(t *testing.T) {
	suffix := time.Now().UnixNano()
	owner := newUser(t, fmt.Sprintf("owner-%d", suffix))
	stranger := newUser(t, fmt.Sprintf("stranger-%d", suffix))
	strangerSession := login(t, stranger.Username)
	{{.Var}} := seed{{.Name}}(t, owner.ID)
	path := fmt.Sprintf("/api/v1/{{.Route}}/%d", {{.Var}}.ID)

	if rec := {{.PluralVar}}Request(t, http.MethodGet, path, strangerSession, nil); rec.Code != http.StatusNotFound {
		t.Errorf("stranger show: %d, want 404", rec.Code)
	}
	if rec := {{.PluralVar}}Request(t, http.MethodPut, path, strangerSession, valid{{.Name}}Request(t, stranger.ID)); rec.Code != http.StatusNotFound {
		t.Errorf("stranger update: %d, want 404", rec.Code)
	}
	if rec := {{.PluralVar}}Request(t, http.MethodDelete, path, strangerSession, nil); rec.Code != http.StatusNotFound {
		t.Errorf("stranger delete: %d, want 404", rec.Code)
	}
	if rec := {{.PluralVar}}Request(t, http.MethodGet, path, login(t, owner.Username), nil); rec.Code != http.StatusOK {
		t.Errorf("the owner's {{.LabelLower}} must survive the stranger: %d", rec.Code)
	}
}
{{- if .HasRequired}}

func Test{{.Plural}}CreateValidation(t *testing.T) {
	owner := newUser(t, fmt.Sprintf("owner-%d", time.Now().UnixNano()))
	rec := {{.PluralVar}}Request(t, http.MethodPost, "/api/v1/{{.Route}}", login(t, owner.Username), map[string]any{})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: %d, want 422; %s", rec.Code, rec.Body)
	}
}
{{- end}}
{{- if .Parent}}

func Test{{.Plural}}UnderStrangers{{.Parent.Name}}(t *testing.T) {
	suffix := time.Now().UnixNano()
	owner := newUser(t, fmt.Sprintf("owner-%d", suffix))
	stranger := newUser(t, fmt.Sprintf("stranger-%d", suffix))
	session := login(t, owner.Username)
	strangers{{.Parent.Name}} := seed{{.Parent.Name}}(t, stranger.ID)

	req := valid{{.Name}}Request(t, owner.ID)
	req.{{.Parent.Field}} = strangers{{.Parent.Name}}.ID
	if rec := {{.PluralVar}}Request(t, http.MethodPost, "/api/v1/{{.Route}}", session, req); rec.Code != http.StatusNotFound {
		t.Errorf("create under the stranger's {{.Parent.LabelLower}}: %d, want 404", rec.Code)
	}
	list := fmt.Sprintf("/api/v1/{{.Route}}?{{.Parent.JSON}}=%d", strangers{{.Parent.Name}}.ID)
	if rec := {{.PluralVar}}Request(t, http.MethodGet, list, session, nil); rec.Code != http.StatusNotFound {
		t.Errorf("list under the stranger's {{.Parent.LabelLower}}: %d, want 404", rec.Code)
	}
}
{{- end}}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: render seeds and integration tests`, plus the trailer.

---

### Task 11: Inspect the project and decide what to bootstrap

**Files:**
- Create: `internal/generate/resource/project.go`, `internal/generate/resource/project_test.go`
- Create the fixture project under `internal/generate/resource/testdata/project/`. Its files are listed in Step 1; `go.sum` is added in Task 14.

**Interfaces:**
- Consumes: `LoadModels`, `buildView`, `view`, `buildSeedModel`, `Resource`.
- Produces:

```go
type decls struct {
	Funcs, Vars, Types map[string]bool
	TypeFiles          map[string]string          // type → declaring file
	Methods            map[string]map[string]bool // receiver type → methods
	Fields             map[string]map[string]bool // struct type → fields
}
func (d decls) has(recv, method string) bool
type Project struct {
	Module, GoMod, Routes, Migrations string
	Models                            ModelIndex
	ModelsPkg, Services, Controllers, ControllerTests decls
}
func Inspect(module string) (*Project, error) // run in the project root
type decisions struct {
	DatabaseService, ValidationService, Helpers, MaxChars, SchemasTest, SetupMigration bool
	Tests                                                                              bool
	Skip                                                                               string
	SetupTest, Harness                                                                 bool
	Seeds                                                                              []*Model
}
func (p *Project) decide(res *Resource, v *view) (*decisions, error)
```

- Also produces these test helpers in `project_test.go`, which Tasks 12–14 reuse: `copyFixture(t) string`, `writeFile(t, path, content)`, `readFile(t, path) string`, `replaceInFile(t, path, old, new)`, `removeFile(t, path)`, `snapshot(t, dir) map[string]string`.

- [ ] **Step 1: Create the fixture project**, a minimal Raptor app with the auth stack.

`testdata/project/.raptor.yaml`:

```yaml
database:
  name: shop
```

`testdata/project/go.mod`:

```
module example.com/shop

go 1.27

require (
	github.com/Oudwins/zog v0.23.0
	github.com/go-raptor/connectors/bun/postgres v1.2.0
	github.com/go-raptor/raptor/v4 v4.4.0
	github.com/uptrace/bun v1.2.18
	golang.org/x/crypto v0.57.0
)
```

`testdata/project/internal/deps/deps.go`:

```go
// Package deps pins modules that code generated into this fixture imports, so go.mod and go.sum
// already cover them before anything is generated.
package deps

import _ "github.com/Oudwins/zog"
```

`testdata/project/config/routes.yaml`:

```yaml
routes:
  /api/v1:
    /auth:
      /login:
        POST: Auth.Login
```

`testdata/project/config/routes.go`:

```go
package config

import (
	_ "embed"

	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/router"
)

//go:embed routes.yaml
var routesYAML []byte

var routes = raptor.Must(router.ParseRoutesYAML(routesYAML))

func Routes() router.Routes { return routes }
```

`testdata/project/config/components/components.go`:

```go
package components

import (
	"example.com/shop/db"
	"github.com/go-raptor/connectors/bun/postgres"
	"github.com/go-raptor/raptor/v4"
)

func New() *raptor.Components {
	return &raptor.Components{
		DatabaseConnector: postgres.NewPostgresConnector(db.MigrationsFS()),
		Services:          Services(),
		Controllers:       Controllers(),
		Middlewares:       Middlewares(),
	}
}
```

`testdata/project/config/components/services.go`:

```go
package components

import (
	"github.com/go-raptor/raptor/v4"
	"example.com/shop/app/services"
)

func Services() raptor.Services {
	return raptor.Services{
		&services.AuthService{},
	}
}
```

`testdata/project/config/components/controllers.go`:

```go
package components

import (
	"github.com/go-raptor/raptor/v4"
	"example.com/shop/app/controllers"
)

func Controllers() raptor.Controllers {
	return raptor.Controllers{
		&controllers.AuthController{},
	}
}
```

`testdata/project/config/components/middlewares.go`:

```go
package components

import "github.com/go-raptor/raptor/v4"

func Middlewares() raptor.Middlewares {
	return raptor.Middlewares{}
}
```

`testdata/project/app/models/user.go`:

```go
package models

import (
	"time"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:users"`

	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	Username  string    `bun:"username,notnull,unique" json:"username"`
	Password  string    `bun:"password,notnull" json:"-"`
	Email     string    `bun:"email,notnull,unique" json:"email"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updatedAt"`
}

func (u *User) ValidPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}
```

`testdata/project/app/models/division.go`:

```go
package models

import "github.com/uptrace/bun"

// Division is shared reference data: rows owned by no one.
type Division struct {
	bun.BaseModel `bun:"table:divisions,alias:divisions"`

	ID   int64  `bun:"id,pk,autoincrement" json:"id"`
	Name string `bun:"name,notnull,unique" json:"name"`
}
```

`testdata/project/app/services/auth_service.go`:

```go
package services

import (
	"example.com/shop/app/models"
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
)

type AuthService struct {
	raptor.Service
}

// CurrentUser stands in for the session lookup in the skill's auth.md; the fixture only has to
// compile.
func (s *AuthService) CurrentUser(ctx *raptor.Context) (*models.User, error) {
	user, ok := ctx.Get("user").(*models.User)
	if !ok {
		return nil, errs.ErrUnauthorized
	}
	return user, nil
}
```

`testdata/project/app/controllers/auth_controller.go`:

```go
package controllers

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
)

type AuthController struct {
	raptor.Controller
}

// Login stands in for the skill's session login; the fixture only has to compile.
func (c *AuthController) Login(ctx *raptor.Context) error {
	return errs.ErrUnauthorized
}
```

`testdata/project/db/migrations.go`:

```go
package db

import (
	"embed"
	"io/fs"
)

//go:embed all:migrations
var migrationsFS embed.FS

func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}
```

`testdata/project/db/migrations/20260101000000_create_users.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id          BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    username    VARCHAR(100) NOT NULL UNIQUE,
    password    TEXT         NOT NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT current_timestamp,
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT current_timestamp
);

CREATE TABLE divisions (
    id    BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    name  VARCHAR(100) NOT NULL UNIQUE
);

-- +goose Down
DROP TABLE IF EXISTS divisions;
DROP TABLE IF EXISTS users;
```

Then run `gofmt -w internal/generate/resource/testdata` so `gofmt -l .` stays clean. The testdata Go files are formatted but never compiled by the cli module.

- [ ] **Step 2: Write the failing tests** (`project_test.go`):

```go
package resource

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyFixture copies testdata/project into a temp dir and changes into it.
func copyFixture(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "project"))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dst)
	return dst
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	src := readFile(t, path)
	if !strings.Contains(src, old) {
		t.Fatalf("%s does not contain %q", path, old)
	}
	writeFile(t, path, strings.ReplaceAll(src, old, new))
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// snapshot maps every file under dir to its content.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		files[filepath.ToSlash(path)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestInspectFixture(t *testing.T) {
	copyFixture(t)
	p, err := Inspect("example.com/shop")
	if err != nil {
		t.Fatal(err)
	}
	if p.Models["User"] == nil || p.Models["Division"] == nil {
		t.Errorf("models: %v", p.Models)
	}
	if !p.Services.has("AuthService", "CurrentUser") {
		t.Error("AuthService.CurrentUser not found")
	}
	if p.Services.TypeFiles["AuthService"] != filepath.Join("app", "services", "auth_service.go") {
		t.Errorf("TypeFiles: %v", p.Services.TypeFiles)
	}
	if !strings.Contains(p.Routes, "Auth.Login") || !strings.Contains(p.Migrations, "create table users") {
		t.Error("routes or migrations not read")
	}
}

func decideFor(t *testing.T, name string, specs []string, parent string) (*decisions, error) {
	t.Helper()
	p, err := Inspect("example.com/shop")
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewResource(name, specs, parent, "", false)
	if err != nil {
		t.Fatal(err)
	}
	v, err := buildView(res, p.Module, p.Models)
	if err != nil {
		return nil, err
	}
	return p.decide(res, v)
}

func TestDecideBootstrapsTheSharedPieces(t *testing.T) {
	copyFixture(t)
	d, err := decideFor(t, "Course", []string{"name:string", "division:ref"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !(d.DatabaseService && d.ValidationService && d.Helpers && d.MaxChars && d.SchemasTest && d.SetupMigration) {
		t.Errorf("bootstrap: %+v", d)
	}
	if !d.Tests || !d.SetupTest || !d.Harness || d.Skip != "" {
		t.Errorf("tests: %+v", d)
	}
	if len(d.Seeds) != 1 || d.Seeds[0].Name != "Division" {
		t.Errorf("seeds: %+v", d.Seeds)
	}
}

func TestDecideRequirements(t *testing.T) {
	tests := []struct {
		name, resource, want string
		setup                func(t *testing.T)
	}{
		{"no bun connector", "Course", "raptor db init postgres --bun", func(t *testing.T) {
			replaceInFile(t, "go.mod", "github.com/go-raptor/connectors/bun/postgres", "github.com/go-raptor/connectors/pgx")
		}},
		{"no auth stack", "Course", "auth stack", func(t *testing.T) { removeFile(t, "app/services/auth_service.go") }},
		{"existing model", "Division", "model Division already exists", func(*testing.T) {}},
		{"partial helpers", "Course", "only some of bindJSON", func(t *testing.T) {
			writeFile(t, "app/controllers/helpers.go", "package controllers\n\nfunc bindJSON() {}\n")
		}},
		{"incomplete DatabaseService", "Course", "DatabaseService lacks Ctx, Conn, HandleError, HandleErrorNotFound, HandleAffected", func(t *testing.T) {
			writeFile(t, "app/services/database_service.go", "package services\n\ntype DatabaseService struct{}\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyFixture(t)
			tt.setup(t)
			_, err := decideFor(t, tt.resource, nil, "")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestDecideSkipsIntegrationTests(t *testing.T) {
	tests := []struct {
		name  string
		specs []string
		setup func(t *testing.T)
	}{
		{"partial harness", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/harness_test.go", "package controllers_test\n\nfunc login() {}\n")
		}},
		{"no login route", nil, func(t *testing.T) { replaceInFile(t, "config/routes.yaml", "Auth.Login", "Auth.SignIn") }},
		{"unseedable ref", []string{"name:string", "kind:ref"}, func(t *testing.T) {
			writeFile(t, "app/models/kind.go", "package models\n\nimport \"github.com/uptrace/bun\"\n\ntype KindCode string\n\ntype Kind struct {\n\tbun.BaseModel `bun:\"table:kinds,alias:kinds\"`\n\n\tID   int64    `bun:\"id,pk,autoincrement\" json:\"id\"`\n\tCode KindCode `bun:\"code,notnull\" json:\"code\"`\n}\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyFixture(t)
			tt.setup(t)
			d, err := decideFor(t, "Course", tt.specs, "")
			if err != nil {
				t.Fatal(err)
			}
			if d.Tests || d.Skip == "" {
				t.Fatalf("integration tests should be skipped with a reason: %+v", d)
			}
		})
	}
}
```

- [ ] **Step 3:** Run `go test ./internal/generate/resource/ -run 'Inspect|Decide'`. Expected: FAIL (undefined `Inspect`).
- [ ] **Step 4: Implement** `project.go`:

```go
package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// decls are one package's top-level declarations, as far as the preconditions need them.
type decls struct {
	Funcs     map[string]bool
	Vars      map[string]bool
	Types     map[string]bool
	TypeFiles map[string]string          // type → the file that declares it
	Methods   map[string]map[string]bool // receiver type → method names
	Fields    map[string]map[string]bool // struct type → field names
}

func (d decls) has(recv, method string) bool { return d.Methods[recv][method] }

// loadDecls reads dir's non-test files, or with tests its _test.go files in an external
// (_test) package.
func loadDecls(dir string, tests bool) (decls, error) {
	d := decls{
		Funcs: map[string]bool{}, Vars: map[string]bool{}, Types: map[string]bool{},
		TypeFiles: map[string]string{}, Methods: map[string]map[string]bool{}, Fields: map[string]map[string]bool{},
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") != tests {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return d, fmt.Errorf("parsing %s: %w", path, err)
		}
		if tests && !strings.HasSuffix(file.Name.Name, "_test") {
			continue
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					d.Funcs[decl.Name.Name] = true
					continue
				}
				recv := strings.TrimPrefix(types.ExprString(decl.Recv.List[0].Type), "*")
				if d.Methods[recv] == nil {
					d.Methods[recv] = map[string]bool{}
				}
				d.Methods[recv][decl.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, n := range spec.Names {
							d.Vars[n.Name] = true
						}
					case *ast.TypeSpec:
						d.Types[spec.Name.Name] = true
						d.TypeFiles[spec.Name.Name] = path
						if st, ok := spec.Type.(*ast.StructType); ok {
							fields := map[string]bool{}
							for _, f := range st.Fields.List {
								for _, n := range f.Names {
									fields[n.Name] = true
								}
							}
							d.Fields[spec.Name.Name] = fields
						}
					}
				}
			}
		}
	}
	return d, nil
}

// Project is what the generator learns about the project before writing anything.
type Project struct {
	Module          string
	GoMod           string
	Routes          string // config/routes.yaml; "" when missing
	Migrations      string // every migration file, lowercased
	Models          ModelIndex
	ModelsPkg       decls
	Services        decls
	Controllers     decls
	ControllerTests decls
}

// Inspect reads the project in the working directory, which must be its root.
func Inspect(module string) (*Project, error) {
	p := &Project{Module: module}
	goMod, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, err
	}
	p.GoMod = string(goMod)
	if routes, err := os.ReadFile(filepath.Join("config", "routes.yaml")); err == nil {
		p.Routes = string(routes)
	}
	if entries, err := os.ReadDir(filepath.Join("db", "migrations")); err == nil {
		var b strings.Builder
		for _, e := range entries {
			if content, err := os.ReadFile(filepath.Join("db", "migrations", e.Name())); err == nil && !e.IsDir() {
				b.WriteString(strings.ToLower(string(content)))
				b.WriteString("\n")
			}
		}
		p.Migrations = b.String()
	}
	if p.Models, err = LoadModels(filepath.Join("app", "models")); err != nil {
		return nil, err
	}
	for _, load := range []struct {
		dst   *decls
		dir   string
		tests bool
	}{
		{&p.ModelsPkg, filepath.Join("app", "models"), false},
		{&p.Services, filepath.Join("app", "services"), false},
		{&p.Controllers, filepath.Join("app", "controllers"), false},
		{&p.ControllerTests, filepath.Join("app", "controllers"), true},
	} {
		if *load.dst, err = loadDecls(load.dir, load.tests); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// decisions records what the generator bootstraps and whether it writes integration tests.
type decisions struct {
	DatabaseService, ValidationService, Helpers, MaxChars, SchemasTest bool
	SetupMigration                                                     bool
	Tests                                                              bool
	Skip                                                               string // why there are no integration tests
	SetupTest, Harness                                                 bool
	Seeds                                                              []*Model // models that need a generated seed function
}

var harnessHelpers = []string{"db", "mustInsert", "newUser", "login", "withSession"}

func countDefined(set map[string]bool, names ...string) int {
	n := 0
	for _, name := range names {
		if set[name] {
			n++
		}
	}
	return n
}

// decide checks the preconditions and works out what to bootstrap. It never touches a file.
func (p *Project) decide(res *Resource, v *view) (*decisions, error) {
	if !strings.Contains(p.GoMod, "github.com/go-raptor/connectors/bun/postgres") {
		return nil, errors.New("the project has no Bun Postgres connector; run `raptor db init postgres --bun` first")
	}
	user := p.Models["User"]
	if user == nil || !user.HasColumn("id") || !p.Services.has("AuthService", "CurrentUser") || !strings.Contains(p.Migrations, "create table users") {
		return nil, errors.New("raptor g resource needs the auth stack (models.User, AuthService.CurrentUser and a users migration); see the raptor-api-conventions skill's references/auth.md")
	}
	if _, exists := p.Models[res.Name]; exists {
		return nil, fmt.Errorf("model %s already exists in app/models", res.Name)
	}

	d := &decisions{}
	var problems []string
	if p.Services.Types["DatabaseService"] {
		var missing []string
		if !p.Services.Fields["DatabaseService"]["Ctx"] {
			missing = append(missing, "Ctx")
		}
		for _, m := range []string{"Conn", "HandleError", "HandleErrorNotFound", "HandleAffected"} {
			if !p.Services.has("DatabaseService", m) {
				missing = append(missing, m)
			}
		}
		if len(missing) > 0 {
			problems = append(problems, "DatabaseService lacks "+strings.Join(missing, ", "))
		}
	} else {
		d.DatabaseService = true
	}
	d.ValidationService = !p.Services.Types["ValidationService"]
	switch countDefined(p.Controllers.Funcs, "bindJSON", "validationFailed", "pathID") {
	case 3:
	case 0:
		d.Helpers = true
	default:
		problems = append(problems, "app/controllers defines only some of bindJSON, validationFailed and pathID")
	}
	d.MaxChars = v.UsesMaxChars && !p.ModelsPkg.Funcs["maxChars"]
	_, err := os.Stat(filepath.Join("app", "models", "schemas_test.go"))
	d.SchemasTest = errors.Is(err, fs.ErrNotExist)
	d.SetupMigration = !strings.Contains(p.Migrations, "set_updated_at()")
	if v.Parent != nil && !p.Services.has(v.Parent.Plural+"Service", "VerifyOwnership") {
		problems = append(problems, fmt.Sprintf("%sService has no VerifyOwnership(id, userID int64) error method, which a child's service calls", v.Parent.Plural))
	}
	for _, fn := range []string{"seed" + res.Name, "valid" + res.Name + "Request", v.PluralVar + "Request"} {
		if p.ControllerTests.Funcs[fn] {
			problems = append(problems, fmt.Sprintf("the controllers tests already define %s", fn))
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("cannot generate %s:\n  - %s", res.Name, strings.Join(problems, "\n  - "))
	}

	if reason := p.decideTests(v, d); reason != "" {
		d.Skip = reason
	} else {
		d.Tests = true
	}
	return d, nil
}

// decideTests returns why integration tests can't be generated, or "" after recording the test
// files to bootstrap.
func (p *Project) decideTests(v *view, d *decisions) string {
	if !strings.Contains(p.Routes, "Auth.Login") {
		return "config/routes.yaml has no Auth.Login route for the tests to log in through"
	}
	switch countDefined(p.ControllerTests.Funcs, harnessHelpers...) {
	case len(harnessHelpers):
	case 0:
		for _, f := range []string{"Username", "Password", "Email"} {
			if _, ok := p.Models["User"].Field(f); !ok {
				return "models.User has no " + f + " field for the generated test harness"
			}
		}
		if p.ControllerTests.Vars["testPassword"] || p.ControllerTests.Vars["clientIPs"] || p.ControllerTests.Funcs["newClient"] {
			return "the controllers tests define some of testPassword, clientIPs and newClient but not the harness around them"
		}
		d.Harness = true
	default:
		return "the controllers tests define only some of " + strings.Join(harnessHelpers, ", ")
	}
	d.SetupTest = !p.ControllerTests.Vars["app"]
	seeds, err := p.seedClosure(v)
	if err != nil {
		return err.Error()
	}
	d.Seeds = seeds
	return ""
}

// seedClosure lists the models whose seed functions the generated tests call but the project
// lacks: the parent chain and every required ref target, followed through their own foreign
// keys.
func (p *Project) seedClosure(v *view) ([]*Model, error) {
	var out []*Model
	seen := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if seen[name] || p.ControllerTests.Funcs["seed"+name] {
			return
		}
		seen[name] = true
		m := p.Models[name]
		out = append(out, m)
		for _, f := range m.Fields {
			if f.Column == "" || f.Column == "user_id" || strings.HasPrefix(f.GoType, "*") {
				continue
			}
			if target, ok := p.Models.FKTarget(m, f.Column); ok {
				visit(target.Name)
			}
		}
	}
	if v.Parent != nil {
		visit(v.Parent.Name)
	}
	for _, r := range v.Refs {
		if !r.Optional {
			visit(r.Model)
		}
	}
	for _, m := range out {
		if _, err := buildSeedModel(p.Models, m, p.Module); err != nil {
			return nil, err
		}
	}
	return out, nil
}
```

- [ ] **Step 5:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 6:** Commit with the message `resource: inspect the project and decide what to bootstrap`, plus the trailer.

---

### Task 12: Edit existing files safely

**Files:**
- Create: `internal/generate/resource/edits.go`, `internal/generate/resource/edits_test.go`
- Modify: `go.mod` and `go.sum` (`go get go.yaml.in/yaml/v3@v3.0.5`)

**Interfaces:**
- Produces:

```go
func addValidationSchema(src, module, name string) (string, error)
func addSchemaTest(src, module, name string) (string, error)
func addRoutes(src, route, controller string) (string, error)
func routesBlock(indent, step int, route, controller string) []string
```

- [ ] **Step 1: Write the failing tests** (`edits_test.go`):

```go
package resource

import (
	"strings"
	"testing"
)

func TestAddValidationSchema(t *testing.T) {
	src := mustRender(t, "validation_service.go.tmpl", mustView(t, "Seminar", nil, "", false))
	out, err := addValidationSchema(src, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "SeminarSchema *zog.StructSchema TopicSchema *zog.StructSchema }")
	loose(t, out, "s.SeminarSchema = models.SeminarSchema() s.TopicSchema = models.TopicSchema() return nil")

	renamed := strings.ReplaceAll(src, "(s *ValidationService)", "(v *ValidationService)")
	renamed = strings.ReplaceAll(renamed, "s.SeminarSchema =", "v.SeminarSchema =")
	out, err = addValidationSchema(renamed, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "v.TopicSchema = models.TopicSchema()")

	for _, broken := range []string{
		"package services\n\ntype ValidationService struct{}\n",
		strings.Replace(src, "\"example.com/shop/app/models\"\n", "", 1),
	} {
		if _, err := addValidationSchema(broken, "example.com/shop", "Topic"); err == nil {
			t.Errorf("an edit it cannot make safely must be reported:\n%s", broken)
		}
	}
}

func TestAddSchemaTest(t *testing.T) {
	src := mustRender(t, "schemas_test.go.tmpl", mustView(t, "Seminar", nil, "", false))
	out, err := addSchemaTest(src, "example.com/shop", "Topic")
	if err != nil {
		t.Fatal(err)
	}
	loose(t, out, "models.SeminarSchema().Validate(&models.SeminarRequest{}) models.TopicSchema().Validate(&models.TopicRequest{}) }")
	if _, err := addSchemaTest("package models_test\n", "example.com/shop", "Topic"); err == nil {
		t.Error("a file without TestSchemasMatchTheirRequests must be reported")
	}
}

func TestAddRoutes(t *testing.T) {
	src := "routes:\n  /api/v1:\n    /auth:\n      /login:\n        POST: Auth.Login\n  /: SPA.Index\n"
	want := "routes:\n  /api/v1:\n    /auth:\n      /login:\n        POST: Auth.Login\n" +
		"    /courses:\n      GET: Courses.Index\n      POST: Courses.Create\n      /{id}:\n        GET: Courses.Show\n        PUT: Courses.Update\n        DELETE: Courses.Destroy\n" +
		"  /: SPA.Index\n"
	got, err := addRoutes(src, "courses", "Courses")
	if err != nil || got != want {
		t.Fatalf("addRoutes:\n got %q\nwant %q\n%v", got, want, err)
	}
}

// Review Focus 3: 4-space indentation, /api/v1 as the last block, no trailing newline.
func TestAddRoutesAdaptsToTheFile(t *testing.T) {
	src := "routes:\n    /api/v1:\n        /auth:\n            /login:\n                POST: Auth.Login"
	got, err := addRoutes(src, "lecture-groups", "LectureGroups")
	if err != nil {
		t.Fatal(err)
	}
	want := "                POST: Auth.Login\n        /lecture-groups:\n            GET: LectureGroups.Index\n            POST: LectureGroups.Create\n            /{id}:\n                GET: LectureGroups.Show\n                PUT: LectureGroups.Update\n                DELETE: LectureGroups.Destroy\n"
	if !strings.HasSuffix(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestAddRoutesRefuses(t *testing.T) {
	for name, tt := range map[string]struct{ src, want string }{
		"no /api/v1": {"routes:\n  /: SPA.Index\n", "no /api/v1: key"},
		"CRLF":       {"routes:\r\n  /api/v1:\r\n", "CRLF"},
		"existing":   {"routes:\n  /api/v1:\n    /courses:\n      GET: Courses.Index\n", "/courses already exists"},
	} {
		if _, err := addRoutes(tt.src, "courses", "Courses"); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: error = %v, want it to mention %q", name, err, tt.want)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/resource/ -run 'Add'`. Expected: FAIL (undefined `addValidationSchema`).
- [ ] **Step 3: Implement.** First run `go get go.yaml.in/yaml/v3@v3.0.5`. Then write `edits.go`:

```go
package resource

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

func importsPath(file *ast.File, path string) bool {
	for _, imp := range file.Imports {
		if p, err := strconv.Unquote(imp.Path.Value); err == nil && p == path {
			return true
		}
	}
	return false
}

type insertion struct {
	at   int
	text string
}

// insertAll applies insertions from the end backwards, so earlier offsets stay valid, then
// gofmt's the result.
func insertAll(src string, ins []insertion) (string, error) {
	sort.Slice(ins, func(i, j int) bool { return ins[i].at > ins[j].at })
	for _, in := range ins {
		src = src[:in.at] + in.text + src[in.at:]
	}
	out, err := format.Source([]byte(src))
	return string(out), err
}

// addValidationSchema adds <name>Schema to ValidationService: a field, and its assignment in Setup
// just before the final return.
func addValidationSchema(src, module, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	if !importsPath(file, "github.com/Oudwins/zog") || !importsPath(file, module+"/app/models") {
		return "", errors.New("the file does not import zog and the models package")
	}
	var fieldsEnd, setupReturn token.Pos
	recv := ""
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == "ValidationService" {
					if st, ok := ts.Type.(*ast.StructType); ok {
						fieldsEnd = st.Fields.Closing
					}
				}
			}
		case *ast.FuncDecl:
			if decl.Name.Name != "Setup" || decl.Recv == nil || decl.Body == nil || len(decl.Recv.List[0].Names) == 0 ||
				strings.TrimPrefix(types.ExprString(decl.Recv.List[0].Type), "*") != "ValidationService" {
				continue
			}
			if n := len(decl.Body.List); n > 0 {
				if ret, ok := decl.Body.List[n-1].(*ast.ReturnStmt); ok {
					setupReturn, recv = ret.Pos(), decl.Recv.List[0].Names[0].Name
				}
			}
		}
	}
	if !fieldsEnd.IsValid() || !setupReturn.IsValid() {
		return "", errors.New("no ValidationService struct with a Setup method that ends in a return")
	}
	return insertAll(src, []insertion{
		{fset.Position(fieldsEnd).Offset, fmt.Sprintf("\t%sSchema *zog.StructSchema\n", name)},
		{fset.Position(setupReturn).Offset, fmt.Sprintf("%s.%sSchema = models.%sSchema()\n\t", recv, name, name)},
	})
}

// addSchemaTest adds the zero-value Validate call to TestSchemasMatchTheirRequests.
func addSchemaTest(src, module, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	if !importsPath(file, module+"/app/models") {
		return "", errors.New("the file does not import the models package")
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "TestSchemasMatchTheirRequests" && fn.Body != nil {
			return insertAll(src, []insertion{{fset.Position(fn.Body.Rbrace).Offset,
				fmt.Sprintf("\tmodels.%sSchema().Validate(&models.%sRequest{})\n", name, name)}})
		}
	}
	return "", errors.New("no TestSchemasMatchTheirRequests function")
}

// routesBlock is the flat CRUD block of SKILL.md → Routes, at the given indentation.
func routesBlock(indent, step int, route, controller string) []string {
	pad := func(level int) string { return strings.Repeat(" ", indent+level*step) }
	return []string{
		pad(0) + "/" + route + ":",
		pad(1) + "GET: " + controller + ".Index",
		pad(1) + "POST: " + controller + ".Create",
		pad(1) + "/{id}:",
		pad(2) + "GET: " + controller + ".Show",
		pad(2) + "PUT: " + controller + ".Update",
		pad(2) + "DELETE: " + controller + ".Destroy",
	}
}

func leadingSpaces(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// addRoutes appends the resource's block to the end of the /api/v1: mapping, matching the file's
// indentation, and refuses anything it cannot do safely. The result must parse as YAML.
func addRoutes(src, route, controller string) (string, error) {
	if strings.Contains(src, "\r") {
		return "", errors.New("routes.yaml uses CRLF line endings")
	}
	lines := strings.Split(strings.TrimRight(src, "\n"), "\n")
	api := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "/api/v1:" {
			api = i
			break
		}
	}
	if api == -1 {
		return "", errors.New("no /api/v1: key in routes.yaml")
	}
	indent, child, last := leadingSpaces(lines[api]), -1, api
	for i := api + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		ind := leadingSpaces(lines[i])
		if ind <= indent {
			break
		}
		if child == -1 {
			child = ind
		}
		if ind == child && trimmed == "/"+route+":" {
			return "", fmt.Errorf("the route /%s already exists under /api/v1", route)
		}
		last = i
	}
	if child == -1 {
		child = indent + 2
	}
	block := routesBlock(child, child-indent, route, controller)
	out := append(append(append([]string{}, lines[:last+1]...), block...), lines[last+1:]...)
	result := strings.Join(out, "\n") + "\n"
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		return "", fmt.Errorf("the edited routes.yaml would not parse: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `resource: edit existing files safely or refuse`, plus the trailer. Include `go.mod` and `go.sum`.

---

### Task 13: Plan, run, and wire the command

**Files:**
- Create: `internal/generate/resource/run.go`, `internal/generate/resource/run_test.go`, `internal/generate/generate_test.go`
- Modify: `internal/generate/generate.go`

**Interfaces:**
- Consumes: everything above, plus `components.AddEntry` (Task 1).
- Produces:

```go
type File struct{ Path, Content string }
type Edit struct {
	Path   string
	Apply  func(src string) (string, error)
	Manual string
}
type Generation struct {
	Files []File
	Edits []Edit
	Notes []string
}
type Options struct {
	Name, Parent, Plural string
	Fields               []string
	Movable              bool
	Module               string
	Now                  time.Time
	Tidy                 func() error // nil skips go mod tidy
	Out                  io.Writer
}
func Plan(res *Resource, p *Project, now time.Time) (*Generation, error)
func Run(o Options) error
// in package generate:
func checkArgs(componentType string, args []string) error
```

- [ ] **Step 1: Write the failing tests.** `run_test.go`:

```go
package resource

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

func run(t *testing.T, o Options) error {
	t.Helper()
	o.Module, o.Now = "example.com/shop", testNow
	if o.Out == nil {
		o.Out = io.Discard
	}
	return Run(o)
}

func TestRunGeneratesAFlatResource(t *testing.T) {
	copyFixture(t)
	var out bytes.Buffer
	if err := run(t, Options{Name: "Course", Fields: []string{"name:string", "division:ref"}, Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"app/models/course.go", "app/services/courses_service.go", "app/controllers/courses_controller.go",
		"app/services/database_service.go", "app/services/validation_service.go", "app/controllers/helpers.go",
		"app/models/validation.go", "app/models/schemas_test.go",
		"app/controllers/courses_controller_test.go", "app/controllers/seed_course_test.go",
		"app/controllers/seed_division_test.go", "app/controllers/setup_test.go", "app/controllers/harness_test.go",
		"db/migrations/20260925115959_setup.sql", "db/migrations/20260925120000_create_courses.sql",
	} {
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	services := readFile(t, "config/components/services.go")
	dbAt, validationAt, coursesAt := strings.Index(services, "DatabaseService"), strings.Index(services, "ValidationService"), strings.Index(services, "CoursesService")
	if !(0 < dbAt && dbAt < validationAt && validationAt < coursesAt) {
		t.Errorf("DatabaseService, ValidationService and CoursesService must register in that order:\n%s", services)
	}
	loose(t, readFile(t, "config/components/controllers.go"), "&controllers.CoursesController{},")
	loose(t, readFile(t, "config/routes.yaml"), "/courses: GET: Courses.Index")
	if strings.Contains(out.String(), "Add these by hand") {
		t.Errorf("no edit should have failed:\n%s", out.String())
	}
}

func TestRunAddsAChildToAGeneratedParent(t *testing.T) {
	copyFixture(t)
	if err := run(t, Options{Name: "Course", Fields: []string{"name:string"}}); err != nil {
		t.Fatal(err)
	}
	if err := run(t, Options{Name: "Outcome", Fields: []string{"name:string"}, Parent: "Course"}); err != nil {
		t.Fatal(err)
	}
	loose(t, readFile(t, "app/services/validation_service.go"), "s.OutcomeSchema = models.OutcomeSchema()")
	loose(t, readFile(t, "app/models/schemas_test.go"), "models.OutcomeSchema().Validate(&models.OutcomeRequest{})")
	loose(t, readFile(t, "app/controllers/seed_outcome_test.go"), "CourseID: seedCourse(t, userID).ID,")
	loose(t, readFile(t, "config/routes.yaml"), "/outcomes: GET: Outcomes.Index")
}

// Review Focus 5: a refused run changes nothing.
func TestRunRefusesWithoutWriting(t *testing.T) {
	copyFixture(t)
	writeFile(t, "app/controllers/courses_controller.go", "package controllers\n")
	before := snapshot(t, ".")
	err := run(t, Options{Name: "Course"})
	if err == nil || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, ".")) {
		t.Error("a refused run must not change any file")
	}

	copyFixture(t)
	if err := run(t, Options{Name: "Course"}); err != nil {
		t.Fatal(err)
	}
	before = snapshot(t, ".")
	if err := run(t, Options{Name: "Course"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a second run must be refused: %v", err)
	}
	if !reflect.DeepEqual(before, snapshot(t, ".")) {
		t.Error("the second run must not change any file")
	}
}

func TestRunPrintsTheEditsItCannotMake(t *testing.T) {
	copyFixture(t)
	writeFile(t, "config/routes.yaml", "routes:\n  /: SPA.Index\n")
	var out bytes.Buffer
	if err := run(t, Options{Name: "Course", Out: &out}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Add these by hand", "/courses:", "No integration tests were generated"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if readFile(t, "config/routes.yaml") != "routes:\n  /: SPA.Index\n" {
		t.Error("a failed edit must leave the file unchanged")
	}
}
```

`internal/generate/generate_test.go`:

```go
package generate

import "testing"

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		typ  string
		args []string
		ok   bool
	}{
		{"resource", []string{"resource", "Course"}, true},
		{"resource", []string{"resource", "Course", "name:string", "division:ref"}, true},
		{"controller", []string{"controller", "Users"}, true},
		{"controller", []string{"controller", "Users", "extra"}, false},
	}
	for _, tt := range tests {
		if err := checkArgs(tt.typ, tt.args); (err == nil) != tt.ok {
			t.Errorf("checkArgs(%q, %v) = %v", tt.typ, tt.args, err)
		}
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/generate/...`. Expected: FAIL (undefined `Run`, `checkArgs`).
- [ ] **Step 3: Implement** `run.go`:

```go
package resource

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-raptor/cli/internal/components"
	"github.com/go-raptor/cli/internal/naming"
)

// File is a new file the generator writes.
type File struct{ Path, Content string }

// Edit changes an existing file. Manual is printed when Apply fails, and the file is left alone.
type Edit struct {
	Path   string
	Apply  func(src string) (string, error)
	Manual string
}

// Generation is everything one run writes.
type Generation struct {
	Files []File
	Edits []Edit
	Notes []string
}

const migrationStamp = "20060102150405"

// Plan renders every new file and prepares every edit, touching nothing on disk.
func Plan(res *Resource, p *Project, now time.Time) (*Generation, error) {
	v, err := buildView(res, p.Module, p.Models)
	if err != nil {
		return nil, err
	}
	d, err := p.decide(res, v)
	if err != nil {
		return nil, err
	}
	g := &Generation{}
	snake := naming.Snake(res.Name)
	for _, s := range []struct {
		ok         bool
		path, tmpl string
	}{
		{true, filepath.Join("app", "models", snake+".go"), "model.go.tmpl"},
		{true, filepath.Join("app", "services", v.Table+"_service.go"), "service.go.tmpl"},
		{true, filepath.Join("app", "controllers", v.Table+"_controller.go"), "controller.go.tmpl"},
		{d.DatabaseService, filepath.Join("app", "services", "database_service.go"), "database_service.go.tmpl"},
		{d.ValidationService, filepath.Join("app", "services", "validation_service.go"), "validation_service.go.tmpl"},
		{d.Helpers, filepath.Join("app", "controllers", "helpers.go"), "helpers.go.tmpl"},
		{d.MaxChars, filepath.Join("app", "models", "validation.go"), "validation.go.tmpl"},
		{d.SchemasTest, filepath.Join("app", "models", "schemas_test.go"), "schemas_test.go.tmpl"},
		{d.Tests, filepath.Join("app", "controllers", v.Table+"_controller_test.go"), "controller_test.go.tmpl"},
		{d.Tests, filepath.Join("app", "controllers", "seed_"+snake+"_test.go"), "seed.go.tmpl"},
		{d.Tests && d.SetupTest, filepath.Join("app", "controllers", "setup_test.go"), "setup_test.go.tmpl"},
		{d.Tests && d.Harness, filepath.Join("app", "controllers", "harness_test.go"), "harness_test.go.tmpl"},
	} {
		if !s.ok {
			continue
		}
		content, err := renderGo(s.tmpl, v)
		if err != nil {
			return nil, err
		}
		g.Files = append(g.Files, File{s.path, content})
	}
	if d.Tests {
		for _, m := range d.Seeds {
			sv, err := buildSeedModel(p.Models, m, p.Module)
			if err != nil {
				return nil, err
			}
			content, err := renderGo("seed_model.go.tmpl", sv)
			if err != nil {
				return nil, err
			}
			g.Files = append(g.Files, File{filepath.Join("app", "controllers", "seed_"+naming.Snake(m.Name)+"_test.go"), content})
		}
	} else {
		g.Notes = append(g.Notes, "No integration tests were generated: "+d.Skip+".")
	}
	stamp := now.UTC()
	if d.SetupMigration {
		g.Files = append(g.Files, File{filepath.Join("db", "migrations", stamp.Add(-time.Second).Format(migrationStamp)+"_setup.sql"), setupMigration})
	}
	g.Files = append(g.Files, File{filepath.Join("db", "migrations", stamp.Format(migrationStamp)+"_create_"+v.Table+".sql"), renderMigration(v)})

	if d.DatabaseService {
		g.Edits = append(g.Edits, registerEdit(p.Module, "service", "DatabaseService"))
	}
	if d.ValidationService {
		g.Edits = append(g.Edits, registerEdit(p.Module, "service", "ValidationService"))
	}
	g.Edits = append(g.Edits,
		registerEdit(p.Module, "service", v.Plural+"Service"),
		registerEdit(p.Module, "controller", v.Plural+"Controller"))
	if !d.ValidationService {
		g.Edits = append(g.Edits, Edit{
			Path:   p.Services.TypeFiles["ValidationService"],
			Apply:  func(src string) (string, error) { return addValidationSchema(src, p.Module, v.Name) },
			Manual: fmt.Sprintf("add %sSchema *zog.StructSchema to ValidationService, and s.%sSchema = models.%sSchema() to its Setup", v.Name, v.Name, v.Name),
		})
	}
	if !d.SchemasTest {
		g.Edits = append(g.Edits, Edit{
			Path:   filepath.Join("app", "models", "schemas_test.go"),
			Apply:  func(src string) (string, error) { return addSchemaTest(src, p.Module, v.Name) },
			Manual: fmt.Sprintf("add models.%sSchema().Validate(&models.%sRequest{}) to TestSchemasMatchTheirRequests", v.Name, v.Name),
		})
	}
	g.Edits = append(g.Edits, Edit{
		Path:   filepath.Join("config", "routes.yaml"),
		Apply:  func(src string) (string, error) { return addRoutes(src, v.Route, v.Plural) },
		Manual: "under /api/v1: add\n" + strings.Join(routesBlock(2, 2, v.Route, v.Plural), "\n"),
	})
	return g, nil
}

func registerEdit(module, kind, structName string) Edit {
	list := "raptor.Services{…}"
	if kind == "controller" {
		list = "raptor.Controllers{…}"
	}
	return Edit{
		Path:   filepath.Join("config", "components", kind+"s.go"),
		Apply:  func(src string) (string, error) { return components.AddEntry(src, module, kind, structName) },
		Manual: fmt.Sprintf("add &%ss.%s{} to %s", kind, structName, list),
	}
}

// Options are the command line of one `raptor g resource` run.
type Options struct {
	Name, Parent, Plural string
	Fields               []string
	Movable              bool
	Module               string
	Now                  time.Time
	Tidy                 func() error // nil skips go mod tidy
	Out                  io.Writer
}

// Run generates the resource in the project in the working directory. Nothing is written unless
// every check passes and no new file exists yet.
func Run(o Options) error {
	res, err := NewResource(o.Name, o.Fields, o.Parent, o.Plural, o.Movable)
	if err != nil {
		return err
	}
	p, err := Inspect(o.Module)
	if err != nil {
		return err
	}
	g, err := Plan(res, p, o.Now)
	if err != nil {
		return err
	}
	var existing []string
	for _, f := range g.Files {
		if _, err := os.Stat(f.Path); err == nil {
			existing = append(existing, f.Path)
		}
	}
	if len(existing) > 0 {
		return fmt.Errorf("these files already exist, so nothing was written:\n  %s", strings.Join(existing, "\n  "))
	}
	for _, f := range g.Files {
		if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f.Path, []byte(f.Content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "Created %s\n", f.Path)
	}
	var manual []string
	for _, e := range g.Edits {
		src, err := os.ReadFile(e.Path)
		var out string
		if err == nil {
			out, err = e.Apply(string(src))
		}
		if err == nil && out != string(src) {
			err = os.WriteFile(e.Path, []byte(out), 0o644)
		}
		if err != nil {
			manual = append(manual, fmt.Sprintf("%s (%v): %s", e.Path, err, e.Manual))
			continue
		}
		fmt.Fprintf(o.Out, "Updated %s\n", e.Path)
	}
	if o.Tidy != nil {
		if err := o.Tidy(); err != nil {
			fmt.Fprintf(o.Out, "go mod tidy failed: %v\n", err)
		}
	}
	for _, n := range g.Notes {
		fmt.Fprintf(o.Out, "\n%s\n", n)
	}
	if len(manual) > 0 {
		fmt.Fprintln(o.Out, "\nAdd these by hand:")
		for _, m := range manual {
			fmt.Fprintf(o.Out, "- %s\n", m)
		}
	}
	fmt.Fprint(o.Out, "\nNext steps:\n  raptor db migrate up\n  DATABASE_NAME=<test database> raptor db migrate up\n  go test ./...\n")
	return nil
}
```

`generate.go` changes:

1. **Flags.** Add them, with an `init`:

```go
var (
	resourceParent  string
	resourceMovable bool
	resourcePlural  string
)

func init() {
	Cmd.Flags().StringVar(&resourceParent, "parent", "", "resource: the model this resource belongs to")
	Cmd.Flags().BoolVar(&resourceMovable, "movable", false, "resource: let an update move it to another parent")
	Cmd.Flags().StringVar(&resourcePlural, "plural", "", "resource: the plural of an irregular name")
}

// checkArgs enforces the argument count per type: a resource takes field specs after its name,
// every other type exactly a name.
func checkArgs(componentType string, args []string) error {
	if componentType == "resource" {
		if len(args) < 2 {
			return errors.New("usage: raptor g resource <Name> [field:type ...] [--parent Model] [--movable] [--plural Name]")
		}
		return nil
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: raptor g %s <Name>", componentType)
	}
	return nil
}

func goModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
```

2. **Help and argument count.** In `Cmd`, set `Args: cobra.MinimumNArgs(2)`. Extend `Long` with:

```
Generate a new controller, service, middleware, model, or a whole resource.

Examples:
  raptor generate controller Users
  raptor g service Auth
  raptor g middleware RateLimit
  raptor g model User
  raptor g resource Course name:string lecture_hours:int division:ref
  raptor g resource Outcome name:string position:int --parent Course
  raptor g resource Unit title:string:80 type:enum:lecture,lab --parent Outcome --movable

Resource fields are name:type[:arg][:optional], with the types string[:length],
text, int, int64, bool, time, enum:a,b,c and ref[:Model].
```

3. **Type switch.** In `generate`, add `"resource"` to the accepted types and to the "Available types" message. Right after that switch, add:

```go
	if err := checkArgs(componentType, args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
```

4. **The resource branch.** Directly after `moduleName` is read, before the name is normalized, add:

```go
	if componentType == "resource" {
		err := resource.Run(resource.Options{
			Name: args[1], Fields: args[2:], Parent: resourceParent, Plural: resourcePlural,
			Movable: resourceMovable, Module: moduleName, Now: time.Now(), Tidy: goModTidy, Out: os.Stdout,
		})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}
```

5. **Imports.** Add `errors`, `os/exec`, `time` and `github.com/go-raptor/cli/internal/generate/resource`.

- [ ] **Step 4:** Run `go test ./... && go vet ./... && gofmt -l .`. Expected: PASS.
- [ ] **Step 5:** Commit with the message `Add raptor g resource`, plus the trailer.

---

### Task 14: Freeze the output (goldens) and prove it compiles

**Files:**
- Create: `internal/generate/resource/golden_test.go`, `internal/generate/resource/compile_test.go`, `internal/generate/resource/testdata/golden/**`
- Modify: `internal/generate/resource/testdata/project/go.mod`, and create its `go.sum`

**Interfaces:**
- Consumes: `Run`, `Options` (Task 13); `copyFixture` and `snapshot` (Task 11).
- Produces: `runScenarios(t) map[string]map[string]string` (scenario → changed path → content).

- [ ] **Step 1: Complete the fixture's module files** (needs network once):

```bash
cd internal/generate/resource/testdata/project && GOWORK=off go mod tidy && GOWORK=off go vet ./... && cd -
```

Expected: `go.sum` is created, `go.mod` gains `// indirect` requirements, and `go vet` prints nothing. If `go mod tidy` drops `github.com/Oudwins/zog`, `internal/deps/deps.go` is missing; restore it and rerun.

- [ ] **Step 2: Write the golden test** (`golden_test.go`):

```go
package resource

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// scenarios build on one another in one project: a flat course, an outcome under it, a movable
// unit two levels down, and a record three levels down.
var scenarios = []struct {
	name string
	opts Options
}{
	{"course", Options{Name: "Course", Fields: []string{"name:string", "lecture_hours:int", "division:ref"}}},
	{"outcome", Options{Name: "Outcome", Fields: []string{"name:string", "criteria:text", "position:int"}, Parent: "Course"}},
	{"unit", Options{Name: "Unit", Fields: []string{"title:string:80", "type:enum:lecture,lab", "starts_at:time:optional", "active:bool"}, Parent: "Outcome", Movable: true}},
	{"record", Options{Name: "Record", Fields: []string{"group_number:int"}, Parent: "Unit"}},
}

// runScenarios generates every scenario into the working directory and returns, per scenario,
// the files it created or changed.
func runScenarios(t *testing.T) map[string]map[string]string {
	t.Helper()
	changed := map[string]map[string]string{}
	for i, s := range scenarios {
		opts := s.opts
		opts.Module, opts.Out = "example.com/shop", io.Discard
		opts.Now = testNow.Add(time.Duration(i) * time.Minute)
		before := snapshot(t, ".")
		if err := Run(opts); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		changed[s.name] = map[string]string{}
		for path, content := range snapshot(t, ".") {
			if before[path] != content {
				changed[s.name][path] = content
			}
		}
	}
	return changed
}

func TestGolden(t *testing.T) {
	golden, err := filepath.Abs(filepath.Join("testdata", "golden"))
	if err != nil {
		t.Fatal(err)
	}
	copyFixture(t)
	for scenario, files := range runScenarios(t) {
		for path, content := range files {
			goldenPath := filepath.Join(golden, scenario, filepath.FromSlash(path)+".golden")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Errorf("%s: no golden file for %s; run with -update and review it", scenario, path)
				continue
			}
			if string(want) != content {
				t.Errorf("%s: %s differs from %s; diff them, and run with -update only if the change is intended", scenario, path, goldenPath)
			}
		}
	}
}
```

The `.golden` suffix keeps `gofmt -l .` and the go tool away from the frozen Go files.

- [ ] **Step 3:** Run `go test ./internal/generate/resource/ -run TestGolden`. Expected: FAIL with "no golden file" for every file.
- [ ] **Step 4:** Run `go test ./internal/generate/resource/ -run TestGolden -update`, then `go test ./internal/generate/resource/ -run TestGolden`. Expected: PASS.
- [ ] **Step 5: Review every golden against the skill** (`~/.claude/skills/raptor-api-conventions/references/patterns.md`, `database.md`, `testing.md`). Check:
  - `unit` (depth 2): `unitsOwnedByUser` equals `patterns.md`'s JOIN chain, character for character.
  - `record` (depth 3): `recordsOwnedByUser` equals `patterns.md`'s depth-3 predicate.
  - `unit` is movable: `ApplyTo` copies `OutcomeID`, `UnitUpdatableColumns` begins with `"outcome_id"`, and `Update` verifies the destination.
  - `outcome` is immutable: its `ToModel` stamps `CourseID`, and neither `ApplyTo` nor the allowlist mentions it.
  - Flat `course`: every query filters on `user_id`, `UserID` is `json:"-"`, and `ToModel(userID)` stamps it.
  - Every migration: FK constraints, an index per FK, CHECKs for ints, the trigger, and a `Down` that drops the table (and `unit_type`).
  - Only the first scenario's goldens contain the bootstrap files and `_setup.sql`. Later scenarios only edit `validation_service.go`, `schemas_test.go`, `services.go`, `controllers.go` and `routes.yaml`.
  - The generated tests seed the parent chain (`seedUnit` calls `seedOutcome`, which calls `seedCourse`) and reference data (`seedDivision`).

  Fix any mismatch in the template or view, never by hand-editing a golden, then rerun Step 4.
- [ ] **Step 6: Write the compile test** (`compile_test.go`):

```go
package resource

import (
	"os"
	"os/exec"
	"testing"
)

// TestGeneratedProjectCompiles generates all four scenarios into the fixture project and vets
// it, which type-checks every generated file, tests included, against the real Raptor, Bun and
// zog APIs.
func TestGeneratedProjectCompiles(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a generated project")
	}
	copyFixture(t)
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	download := exec.Command("go", "mod", "download")
	download.Env = env
	if out, err := download.CombinedOutput(); err != nil {
		t.Skipf("the fixture's modules are unavailable (offline?): %v\n%s", err, out)
	}
	runScenarios(t)
	vet := exec.Command("go", "vet", "./...")
	vet.Env = env
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("go vet on the generated project: %v\n%s", err, out)
	}
}
```

- [ ] **Step 7: Prove the compile test catches an API mistake.** Temporarily change `"zog.Ptr(zog.Time())"` in `view.go` to `"zog.Pointer(zog.Time())"`, then run `go test ./internal/generate/resource/ -run TestGeneratedProjectCompiles`. Expected: FAIL, naming `zog.Pointer` as undefined. Revert the change, rerun, and expect PASS. If the test fails for any other reason, the templates don't match the real APIs: debug with superpowers:systematic-debugging, then fix the template and regenerate the goldens (Step 4).
- [ ] **Step 8:** Run `go test ./... && go test -short ./... && go vet ./... && gofmt -l .`. Expected: PASS. `-short` skips only the compile test.
- [ ] **Step 9:** Commit with the message `resource: freeze the generated output and vet it against the real APIs`, plus the trailer.

---

### Task 15: Documentation and the skill

**Files:**
- Modify: `README.md` and `CHANGELOG.md` (cli)
- Modify: `~/.claude/skills/raptor-api-conventions/SKILL.md` (a separate repo, `h00s/claude-skills`)

- [ ] **Step 1: cli README.** Add a `## Generating a resource` section after `## Generated configuration`:

````markdown
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
````

- [ ] **Step 2: cli CHANGELOG.** Add above `## v1.2.0`:

```markdown
## Unreleased (v1.3.0)

### Added

- `raptor g resource <Name> [field:type ...] [--parent Model] [--movable] [--plural Name]` scaffolds a complete entity (model, DTOs, zog schema, service with ownership checks, controller, routes, migration and integration tests), bootstrapping the shared services, helpers and test harness on first use. See the README.
```

- [ ] **Step 3:** Run `go test ./... && go vet ./... && gofmt -l .`, then commit with the message `Document raptor g resource`, plus the trailer.
- [ ] **Step 4: Skill, RED.** Use superpowers:writing-skills. Dispatch a fresh subagent with this prompt:

  > You are adding a new `Seminar` entity (name, lecture_hours, division reference) to a go-raptor API that follows `/home/h00s/.claude/skills/raptor-api-conventions/`. Read SKILL.md and the references it points to. Say, in under 150 words, the first command or file you would write.

  Record its answer. The baseline expectation is that it hand-writes `app/models/seminar.go`.
- [ ] **Step 5: Skill, GREEN.** In `SKILL.md`'s scaffolding list, insert this as the first bullet:

```markdown
- `raptor g resource <Name> field:type… [--parent X] [--movable]` (CLI v1.3.0+) scaffolds a whole entity exactly as `patterns.md` writes it: model, DTOs, schema, service with ownership checks, controller, routes, migration and integration tests. Run it first for every new entity, then refine what it cannot know: sort order, extra rules, CHECKs and relations to load. Field types are `string[:n]`, `text`, `int`, `int64`, `bool`, `time`, `enum:a,b` and `ref[:Model]` (shared reference data only), each optionally `:optional`.
```

  Rerun the Step 4 prompt with a fresh subagent. Expected: it runs `raptor g resource Seminar name:string lecture_hours:int division:ref` first. If it doesn't, tighten the bullet's wording and rerun.
- [ ] **Step 6:** Commit in `~/.claude/skills` with the message `raptor-api-conventions: scaffold new entities with raptor g resource`, plus the trailer. **Don't push**; ask the user.

---

### Task 16: End-to-end check against a real database (needs the user)

The compile test proves the generated code type-checks. Only a real run proves the migrations apply and the generated integration tests pass against Postgres with the full session auth stack, which the fixture only stubs.

- [ ] **Step 1: Ask the user** which app with the auth stack and a test database to try this on, such as a throwaway branch of an existing app. **Stop here until they answer.**
- [ ] **Step 2:** In that app, on a new branch:
  1. Build the cli from this branch: `go build -o /tmp/raptor-dev ./cmd/raptor`.
  2. Run `/tmp/raptor-dev g resource Seminar name:string lecture_hours:int`, then `/tmp/raptor-dev g resource SeminarNote body:text --parent Seminar`.
  3. Run `DATABASE_NAME=<test db> raptor db migrate up`, then `go test ./app/...`.

  Expected: the migrations apply, and `TestSeminars*` and `TestSeminarNotes*` all pass.
- [ ] **Step 3: Report the result to the user.** Then delete the branch or keep it, as they choose.

---

## Verification (end to end)

1. In `~/dev/go/go-raptor/cli`, run `go test ./... && go test -short ./... && go vet ./... && gofmt -l .`. All pass, and `gofmt` prints nothing.
2. Every Review Focus item has its test:
   - 1 (`Type`, `Err`, `List`, an existing `User`): `TestNewResourceErrors` and `TestDecideRequirements`;
   - 2: the `Lesson` row in `TestOwnerChainPredicate`;
   - 3: `TestAddRoutesAdaptsToTheFile` and `TestAddRoutesRefuses`;
   - 4: `TestBuildViewMultiwordChild` and `TestFieldNames`;
   - 5: `TestRunRefusesWithoutWriting`.
3. `TestGeneratedProjectCompiles` passes without `-short`.
4. Task 16 has run against a real database with the user's go-ahead, or its absence is reported.
5. `git log --oneline` shows one commit per task, and nothing is pushed or tagged.

## Plan self-review notes

- **Spec coverage:**
  - command and flags: Tasks 3 and 13;
  - field grammar and type table: Tasks 3 and 6;
  - ownership (flat, child at any depth, immutable or movable): Tasks 5–8;
  - generated files: Tasks 7–10;
  - edits: Tasks 12–13;
  - bootstrap: Tasks 9 and 11;
  - preconditions: Task 11;
  - generated tests, seeds and harness: Tasks 9–11;
  - the generator's structure: the File Structure section;
  - testing the generator: Tasks 3–14;
  - skill and documentation changes: Task 15;
  - manual end-to-end: Task 16.
- **Where the plan refines the spec:**
  - The table in the spec's *What it generates* lists `seed_course_test.go` with the signature `seedCourse(t, userID)`. The plan also generates seed files for models it didn't write (`seed_model.go.tmpl`), as the spec's *Seeding* paragraph requires.
  - The harness also bootstraps `setup_test.go` when the `controllers_test` package lacks `app`. The spec implies this, because the tests need `TestMain`.
  - The spec's table of the generator's own files names `inspect.go` and `apply.go`. This plan splits the work by responsibility instead: `models.go` (the model index), `project.go` (inspection and preconditions), `edits.go` and `run.go` (planning and applying). The File Structure table is authoritative.
