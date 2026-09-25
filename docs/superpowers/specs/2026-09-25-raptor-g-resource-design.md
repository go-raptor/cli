# `raptor g resource` — design

**Date:** 2026-09-25
**Repository:** `go-raptor/cli` (target release v1.3.0)
**Conventions it emits:** the `raptor-api-conventions` skill: `SKILL.md`, `references/patterns.md`, `database.md` and `testing.md`, at commit `f17639e` of `h00s/claude-skills`.

## Goal

One command scaffolds a complete, convention-exact entity. Humans use it to start a new resource; the skill tells agents to run it first and then refine the output, instead of transcribing `patterns.md` by hand. Hand-written layers drift: a field added to the model but not to `ApplyTo`, a Required missing on a schema, an ownership predicate written slightly too loose. Generating every layer from one field list removes that drift.

**Success looks like:**
- `raptor g resource Course name:string lecture_hours:int division:ref` on a project with the auth stack produces code that compiles, passes `go vet`, and after `raptor db migrate up` passes its own generated integration tests.
- The generated files are indistinguishable in shape from `patterns.md`'s Course, Outcome and Unit, including the ownership predicate at depth 2 and deeper.
- The command never half-applies: it either writes every new file or none, and every edit to an existing file is either made cleanly or printed as a manual step.

## Decisions (from the design discussion)

| Question | Decision |
|---|---|
| Audience | Humans and agents. One opinionated scaffold; the skill directs agents to it. |
| Field input | Field specs on the command line; with none, a single `name:string`. |
| Ownership shapes | A flat, user-owned resource by default; `--parent X` makes a child at any depth. |
| Missing plumbing | Bootstrap the shared services and helpers; require the auth stack and stop with a clear message without it. |
| Extras (`--reference`, `--create-only`, SvelteKit side) | **Not in v1** — the question went unanswered, so this spec takes the smaller scope. Each can follow as its own change. |

## Command

```
raptor g resource <Name> [field ...] [--parent <Model>] [--movable] [--plural <Plural>]
```

- `<Name>` is the singular model name in PascalCase (`Course`, `LectureGroup`). A trailing `Resource` is not stripped; the name is used as given.
- `--parent <Model>` makes the resource a child of an existing model (see *Ownership*).
- `--movable` allows re-parenting on update (only with `--parent`). Without it the parent is immutable, the default `patterns.md` recommends.
- `--plural <Plural>` overrides the pluralization of irregular names (`--plural Criteria`). The plural drives the Go plural names (`Courses`, `CoursesService`), the table (`courses`) and the route (`/courses`). Default rules: `+s`; `s`, `x`, `z`, `ch`, `sh` → `+es`; consonant + `y` → `ies`.

### Field specs

`name:type[:arg][:optional]`. Colons and commas only, so every spec is shell-safe without quoting in bash and zsh.

- `name` is snake_case (`lecture_hours`), starts with a letter, and must not be `id`, `user_id`, `created_at`, `updated_at`, the parent's FK column, or a reserved SQL word (`user`, `order`, `group`, `table`, `select`, `default`, `check`, `references`, `limit`, `offset`, `desc`, `asc`, `primary`, `foreign`, `unique`, `end`, `from`, `where`, `to`).
- Go field names apply common initialisms: `url` → `URL`, `id` → `ID`, `api` → `API`, `ip` → `IP`, `uuid` → `UUID`, `html` → `HTML`, `http` → `HTTP`, `json` → `JSON`.
- JSON member names are camelCase (`lectureHours`); columns keep the snake_case name.

| Spec | Go (required / optional) | Column | zog (required / optional) |
|---|---|---|---|
| `x:string[:n]` (n = 150) | `string` / `string` + `nullzero` | `VARCHAR(n) NOT NULL` / `VARCHAR(n)` | `String().Trim().Min(1).TestFunc(maxChars(n)).Required()` / `String().Trim().TestFunc(maxChars(n))` |
| `x:text` | `string` (prose; always optional) | `TEXT NOT NULL DEFAULT ''` | `String()` |
| `x:int` / `x:int64` | `int` / `int64`; optional → pointer | `INTEGER` / `BIGINT` `NOT NULL DEFAULT 0` + `CHECK (x >= 0)`; optional → nullable | `Int().GTE(0)` / `Int64().GTE(0)` (0 is valid, so never Required); optional → `Ptr(…)` |
| `x:bool` | `bool`; optional → `*bool` | `BOOLEAN NOT NULL DEFAULT false`; optional → nullable | `Bool()` (never Required); optional → `Ptr(Bool())` |
| `x:time` | `time.Time`; optional → `*time.Time` | `TIMESTAMPTZ NOT NULL`; optional → nullable | `Time().Required()`; optional → `Ptr(Time())` |
| `x:enum:a,b,c` | typed string `<Model><X>` (`UnitType`) + constants + `<Model><X>Values`; optional → `nullzero` | `CREATE TYPE <model>_<x> AS ENUM ('a','b','c')` (`unit_type`) in the table's own migration, dropped in its `Down`; `NOT NULL` / nullable | `String().OneOf(models.UnitTypeValues).Required()` / without `Required` |
| `x:ref[:Model]` (Model = PascalCase of x) | `XID int64` + `X *Model` belongs-to relation `json:"-"`; optional → `*int64` | `x_id BIGINT NOT NULL` + FK `ON DELETE RESTRICT` + index; optional → nullable | `Int64().GT(0).Required()`; optional → `Ptr(Int64().GT(0))` |

- `optional` on `text` is an error ("text is already optional").
- `ref` is for **shared reference data** (rows owned by no one, like Division). If the target model is user-owned (see *Ownership*), the command fails: "Course is owned by a user; use --parent Course, or the ownership check would be skipped." A `ref` never gets an ownership check, so allowing it on owned data would let one user attach another user's rows.
- The first required `string` field is the list order key (`ORDER BY <table>.<field>, <table>.id`); without one, lists order by `id`.

## Ownership

**Flat (default).** The model gets `UserID int64 bun:"user_id,notnull" json:"-"`. `ToModel(userID)` stamps it, every service method filters on `user_id = ?`, and the migration has `user_id BIGINT NOT NULL` with `FOREIGN KEY … REFERENCES users (id) ON DELETE CASCADE` plus an index.

**Child (`--parent P`).** The generator reads `app/models/*.go` with `go/parser` and builds a model index: struct name, `bun.BaseModel` table, and each field's Go name, type and bun column. From it, it derives P's **ownership chain**:
- A model is **root-owned** if it has a `user_id` column.
- Otherwise it is **child-owned** through the one FK column (an `x_id` field whose `x` names an indexed model, or a `rel:belongs-to,join:x_id=id` relation) whose target is itself owned. The walk repeats up to a root-owned model.
- **Reference data** is a model that is neither.
- Zero owned FKs means P isn't owned and the command fails. Two or more owned FKs is ambiguous and also fails, naming the candidates (v1 has no flag to choose).

The child gets `<P>ID int64` (`json:"<p>Id"`, `NOT NULL`, FK `ON DELETE CASCADE`, index) and an ownership predicate constant, exactly as `patterns.md` writes them:

- **Parent root-owned (depth 1):**
  `outcomes.course_id IN (SELECT id FROM courses WHERE user_id = ?)`
- **Deeper (depth ≥ 2):** a JOIN chain from the parent to the root:
  ```
  units.outcome_id IN (
      SELECT outcomes.id FROM outcomes
      JOIN courses ON courses.id = outcomes.course_id
      WHERE courses.user_id = ?
  )
  ```

The child's service injects the parent's service (`Courses *CoursesService`) and calls its `VerifyOwnership(parentID, userID)` on create and on list-by-parent. With `--movable` it also calls it on update, for the destination. P's service must declare `VerifyOwnership(int64, int64) error`; generated services always do, so any generated resource can become a parent.

**Parent immutability.** By default `ToModel()` stamps the parent FK from the request, `ApplyTo` never copies it, and it's absent from `XUpdatableColumns`. With `--movable`, `ApplyTo` copies it, the allowlist includes it, and `Update` verifies the destination, as in the Outcome template.

## What it generates

For `raptor g resource Course name:string lecture_hours:int division:ref`:

| File | Content |
|---|---|
| `app/models/course.go` | Model, `Courses` alias, `CourseRequest`, `CourseSchema()`, `ToModel`, `ApplyTo`, `CourseUpdatableColumns`, `CourseResponse`, `NewCourseResponse(s)`; enum types when present |
| `app/services/courses_service.go` | `List`, (`ListBy<Parent>` for a child), `Get`, `Create`, `Update`, `Delete`, `VerifyOwnership`; the ownership predicate constant for a child |
| `app/controllers/courses_controller.go` | `Index` (with `?<parent>Id=` for a child), `Show`, `Create`, `Update`, `Destroy`, following SKILL.md → *Controllers* step by step |
| `db/migrations/<ts>_create_courses.sql` | Named like `raptor db migrate create` output. The `database.md` table template: table, FKs, CHECKs, indexes, `updated_at` trigger; `Down` drops the table (and its enum types) |
| `app/controllers/courses_controller_test.go` | Integration tests (see *Generated tests*) |
| `app/controllers/seed_course_test.go` | `seedCourse(t *testing.T, userID int64) *models.Course`, reused by children's tests |

**Edits to existing files.** Each edit is anchored on a known marker, and the result is gofmt'd (Go) or parsed (YAML) before it is written. If an anchor is missing or the result doesn't parse, the file is left untouched and the exact lines to add are printed.
- `config/components/services.go` and `controllers.go`: register `CoursesService` / `CoursesController`, via the existing `registerComponent`.
- `app/services/validation_service.go`: add `CourseSchema *zog.StructSchema` and `s.CourseSchema = models.CourseSchema()` in `Setup`.
- `app/models/schemas_test.go`: add `models.CourseSchema().Validate(&models.CourseRequest{})`.
- `config/routes.yaml`: add the resource block under `/api/v1:`, in the flat shape SKILL.md → *Routes* shows; the route segment is the kebab-case plural (`/lecture-groups`). The edit is text insertion after the last line of the `/api/v1:` mapping, verified by parsing the result with `go.yaml.in/yaml/v3`.

**Bootstrap (only when missing).** Each file is created from `patterns.md` / `testing.md` verbatim, apart from the module path:
- `app/services/database_service.go` and `app/services/validation_service.go`, registered first in `services.go` (`DatabaseService` before everything else);
- `app/controllers/helpers.go` (`bindJSON`, `validationFailed`, `pathID`);
- `app/models/validation.go` (`maxChars`);
- `app/models/schemas_test.go`;
- `app/controllers/harness_test.go` (the `testing.md` fixtures and request helpers, see *Generated tests*);
- a `db/migrations/<ts>_setup.sql` with `set_updated_at()`, when no migration defines it. Its timestamp is one second earlier than the table migration's, so Goose applies it first.

After writing, the command runs `go mod tidy` (as `db init` does) so `github.com/Oudwins/zog` lands in `go.mod`. It then prints next steps:

```
raptor db migrate up
DATABASE_NAME=<name>_test raptor db migrate up
go test ./...
```

## Preconditions (checked before anything is written)

1. The working directory is inside a Raptor project (`project.FindRoot`), and `go.mod` names the module.
2. `go.mod` requires `github.com/go-raptor/connectors/bun/postgres`. Otherwise: "run `raptor db init postgres --bun` first".
3. The auth stack exists: `models.User` with an `ID int64` field, `AuthService` with a `CurrentUser` method, and a migration that creates `users`. Otherwise stop, pointing to the skill's `auth.md`.
4. Existing shared files provide what the generated code calls:
   - `DatabaseService` has `Ctx`, `Conn`, `HandleError`, `HandleErrorNotFound` and `HandleAffected`;
   - `helpers.go` has `bindJSON`, `validationFailed` and `pathID`;
   - `maxChars` exists when any string field is present.

   A missing member stops the command with a list. It never edits user code to add them.
5. `--parent` and `ref` targets resolve in the model index, as described under *Ownership*.
6. No file the command would create already exists. There is no `--force`: a partially generated resource is worse than a clear refusal.

Everything is rendered in memory first. New files are written only after all checks pass. Edits to existing files come last and are individually best-effort, as described above.

## Generated tests

**Always:** the schema line in `schemas_test.go`. It catches a zog type mismatch, which otherwise panics inside `Validate`.

**Integration tests** (`<plural>_controller_test.go`), driving the real router and database:
- no session → 401 on `GET /api/v1/<route>`;
- owner: create → 201, show → 200, update → 200, list includes it, delete → 204, show → 404;
- stranger: show, update and delete of the owner's row → 404, and the row is unchanged afterwards;
- `{}` → 422, when any field is `Required`;
- child: create under the stranger's parent → 404, and list with `?<parent>Id=` of the stranger's parent → 404.

**Seeding** uses generated `seed<Model>(t, …)` functions, one per model and one file each. A seed fills fields with sample values by type:
- strings: `"Sample <field> <suffix>"`;
- `int`: 1;
- `bool`: true;
- `time`: a fixed date;
- enums: the first value;
- optional fields: zero or nil.

FK fields call the target's seed first:
- an owned model's seed is `seed<Model>(t, userID int64) *models.<Model>`, and a child's seed builds a fresh parent chain for that user, so `seedOutcome(t, strangerID)` yields a whole tree the stranger owns;
- a reference model's seed is `seed<Model>(t) *models.<Model>`, used for `ref`s.

The root gets the user id, and `t.Cleanup` deletes what it inserted. Deleting the users cascades through owned trees; reference rows are deleted last. If a model in that FK closure has no seed function yet (it wasn't generated), one is generated from its fields when every field type is one the generator knows. Otherwise the integration test file is skipped and the command says which model blocked it.

**Harness.** The generated tests call only five helpers, all as `testing.md` defines them:
- `db(t) *bun.DB`;
- `mustInsert(t, *bun.InsertQuery)`;
- `newUser(t, username) *models.User`;
- `login(t, username) *http.Cookie`;
- `withSession(*http.Cookie) raptor.TestRequestOption`.

Requests go through `app.TestGet`, `TestPost`, `TestPut` and `TestDelete`, with bodies marshaled inline. `harness_test.go` (those five, plus `newClient` using `raptor.WithRemoteAddr` and `testPassword`, as in `testing.md`) is bootstrapped only when the `controllers_test` package declares none of them. If the package declares all five with those signatures, the generated tests reuse them. If it declares only some, the integration tests are skipped with a message naming the missing helpers.

## The generator's own structure

A new package `internal/generate/resource`, called from `generate.go`'s type switch. `resource` joins `controller|service|middleware|model`. Its argument count is variadic, so `Args` becomes `MinimumNArgs(2)`, and the other four types check for exactly two arguments themselves, keeping today's usage error:

| File | Responsibility |
|---|---|
| `spec.go` | Parse the name, field specs and flags into a `Resource` value. Pure. |
| `naming.go` | Singular/plural, snake, camel, kebab and initialisms. The existing `toSnakeCase`/`toPascalCase` move here and `generate.go` imports them. |
| `inspect.go` | Read the project: `go.mod` requirements, the model index (`go/parser`), the shared files' members, and existing test helpers. |
| `ownership.go` | Derive the chain and predicate from the model index. Pure over the index. |
| `render.go` + `templates/*.tmpl` | `text/template` rendering (embedded) into a `map[path]content`; Go output is run through `go/format`. |
| `apply.go` | Preconditions, writing new files, anchored edits, `go mod tidy`, and the printed summary. |

The existing generators keep their behavior. `generate model` stays the bare stub; `resource` is the convention-exact path.

## Testing the generator

- **`spec_test.go`:** table tests for field parsing, including every type, `optional`, lengths, enum values, reserved names, `optional` on `text`, and an unknown type, each with the exact error.
- **`ownership_test.go`:** a model index built from fixture sources; chains at depths 1, 2 and 3; reference-data detection; the owned-`ref` rejection; an ambiguous chain; a missing parent.
- **Golden tests:** render scenarios into `testdata/golden/<scenario>/`:
  - flat with every field type;
  - child at depth 1;
  - movable child;
  - child at depth 2;
  - child at depth 3.

  An `-update` flag rewrites the goldens.
- **Compile test:** copy `testdata/project` (a minimal module with the auth stack, components, routes, a `users` migration and `bun/postgres` in `go.mod`) to a temp dir, run the generator for a flat resource plus a depth-2 child, then run `go vet ./...`. The test is skipped under `-short`, and skipped rather than failed when module downloads are unavailable.
- **Manual end-to-end** (in the implementation plan): a scratch project with a local Postgres runs the generated migrations and the generated integration tests.

## Skill and documentation changes

- **`raptor-api-conventions`:**
  - SKILL.md's scaffolding list gains `raptor g resource <Name> field:type… [--parent X]`, the first step for any new entity. Agents then refine sort order, extra rules and CHECKs by hand; `patterns.md` stays the explanation of what the generator emits.
  - The skill's version floor becomes CLI v1.3.0.
- **cli:** a README section on the field-spec grammar and examples, and a CHANGELOG entry for v1.3.0.

## Out of scope for v1

- `--reference` (read-only shared data seeded by migration) and `--create-only` (no update, UNIQUE constraint for 409 on repeat).
- The SvelteKit counterpart (types, zod schema, API service).
- `raptor g auth` (the session stack).
- PATCH, pagination, nested routes, soft delete, UNIQUE modifiers, custom CHECK bounds beyond `>= 0`, and collations. These are hand edits after generation.
- Adding fields to an already generated resource, and any undo or destroy command.

## Risks

- **Anchored edits against hand-edited files.** Mitigated by the parse-or-print rule: a failed edit never corrupts a file.
- **Model-index heuristics** (FK naming, belongs-to tags) on hand-written models. Mitigated by failing loudly on ambiguity and by the ownership tests.
- **Sample values vs hand-written CHECKs** in seeds. A seed insert that violates a custom constraint fails the test with the constraint name, which points at the seed to adjust.
