package resource

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// packageDir is this source file's directory, resolved at compile time so it stays correct even
// after a test has changed the process's working directory with t.Chdir.
var packageDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}()

// copyFixture copies testdata/project into a temp dir and changes into it.
func copyFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join(packageDir, "testdata", "project")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
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
		// Finding I-8: the actual reason, not "already exists".
		{"validation.go without maxChars", "Course", "app/models/validation.go exists but does not declare maxChars", func(t *testing.T) {
			writeFile(t, "app/models/validation.go", "package models\n\nfunc other() {}\n")
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
		want  string   // in the reason; "" accepts any
		not   []string // helpers the reason must not name
	}{
		// Finding I-5: the reason names exactly the helpers that are missing or differ.
		{"partial harness", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/harness_test.go", harness(map[string]string{"db": "", "mustInsert": "", "newUser": ""}))
		}, "the controllers tests' harness does not match what the generated tests call: it lacks db, mustInsert, newUser", []string{"login", "withSession"}},
		{"harness signature", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/harness_test.go", harness(map[string]string{
				"newUser": "func newUser(t *testing.T) *models.User { return nil }",
			}))
		}, "the controllers tests' harness does not match what the generated tests call: newUser is func(*testing.T) *models.User, not func(*testing.T, string) *models.User", []string{"lacks", "db", "mustInsert", "login", "withSession"}},
		{"harness helper that is not a function", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/harness_test.go", harness(map[string]string{"login": "var login = 1"}))
		}, "the controllers tests' harness does not match what the generated tests call: login is not a function", []string{"lacks", "db", "mustInsert", "newUser", "withSession"}},
		{"no login route", nil, func(t *testing.T) { replaceInFile(t, "config/routes.yaml", "Auth.Login", "Auth.SignIn") }, "Auth.Login", nil},
		{"unseedable ref", []string{"name:string", "kind:ref"}, func(t *testing.T) {
			writeFile(t, "app/models/kind.go", "package models\n\nimport \"github.com/uptrace/bun\"\n\ntype KindCode string\n\ntype Kind struct {\n\tbun.BaseModel `bun:\"table:kinds,alias:kinds\"`\n\n\tID   int64    `bun:\"id,pk,autoincrement\" json:\"id\"`\n\tCode KindCode `bun:\"code,notnull\" json:\"code\"`\n}\n")
		}, "no sample value for type KindCode", nil},
		// Finding I-8: never a second TestMain, wherever the first one is.
		{"TestMain without app", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/main_test.go", "package controllers_test\n\nvar testApp int\n\nfunc TestMain(m *testing.M) {}\n")
		}, "app/controllers/main_test.go has a TestMain, but the controllers tests declare no app variable", nil},
		{"TestMain in the internal test package", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/main_internal_test.go", "package controllers\n\nfunc TestMain(m *testing.M) {}\n")
		}, "app/controllers/main_internal_test.go has a TestMain", nil},
		{"setup_test.go without app", nil, func(t *testing.T) {
			writeFile(t, "app/controllers/setup_test.go", "package controllers_test\n\nvar testApp int\n")
		}, "app/controllers/setup_test.go already exists, but the controllers tests declare no app variable", nil},
		// Finding I-1: the harness sets the user's fields in a composite literal, which can't
		// reach fields promoted from an embedded struct.
		{"user fields from a mixin", nil, func(t *testing.T) {
			writeFile(t, "app/models/user.go", userWithCredentials)
		}, "models.User's Username comes from the embedded Credentials", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyFixture(t)
			tt.setup(t)
			d, err := decideFor(t, "Course", tt.specs, "")
			if err != nil {
				t.Fatal(err)
			}
			if d.Tests || d.Skip == "" || !strings.Contains(d.Skip, tt.want) {
				t.Fatalf("integration tests should be skipped with a reason mentioning %q: %+v", tt.want, d)
			}
			for _, name := range tt.not {
				if strings.Contains(d.Skip, name) {
					t.Errorf("the reason names %s, which is neither missing nor different: %s", name, d.Skip)
				}
			}
		})
	}
}

// harness is a controllers_test file declaring the five helpers the generated tests call, with
// the spec's signatures but other parameter names. overrides replaces a helper's declaration, or
// with "" leaves it out.
func harness(overrides map[string]string) string {
	decls := []struct{ name, decl string }{
		{"db", "func db(tb *testing.T) *bun.DB { return nil }"},
		{"mustInsert", "func mustInsert(tb *testing.T, query *bun.InsertQuery) {}"},
		{"newUser", "func newUser(tb *testing.T, name string) *models.User { return nil }"},
		{"login", "func login(tb *testing.T, name string) *http.Cookie { return nil }"},
		{"withSession", "func withSession(cookie *http.Cookie) raptor.TestRequestOption { return nil }"},
	}
	src := "package controllers_test\n"
	for _, d := range decls {
		if decl, ok := overrides[d.name]; ok {
			d.decl = decl
		}
		if d.decl != "" {
			src += "\n" + d.decl + "\n"
		}
	}
	return src
}

// Finding I-5: a harness with the spec's signatures is reused, whatever its parameter names.
func TestDecideReusesAMatchingHarness(t *testing.T) {
	copyFixture(t)
	writeFile(t, "app/controllers/harness_test.go", harness(nil))
	d, err := decideFor(t, "Course", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Tests || d.Harness {
		t.Fatalf("the harness should be reused: %+v", d)
	}
}

const userWithCredentials = `package models

import "github.com/uptrace/bun"

type Credentials struct {
	Username string ` + "`" + `bun:"username,notnull,unique" json:"username"` + "`" + `
	Password string ` + "`" + `bun:"password,notnull" json:"-"` + "`" + `
	Email    string ` + "`" + `bun:"email,notnull,unique" json:"email"` + "`" + `
}

type User struct {
	bun.BaseModel ` + "`" + `bun:"table:users,alias:users"` + "`" + `

	ID int64 ` + "`" + `bun:"id,pk,autoincrement" json:"id"` + "`" + `
	Credentials
}
`

// Finding I-3: a generated package-level name must not clash with anything the package already
// declares, Bun model or not.
func TestDecideRefusesNameClashes(t *testing.T) {
	tests := []struct {
		name, resource, parent, want string
		setup                        func(t *testing.T)
	}{
		{"non-Bun type", "Report", "", "app/models already declares Report", func(t *testing.T) {
			writeFile(t, "app/models/report.go", "package models\n\ntype Report struct{ Title string }\n")
		}},
		{"response constructor", "Report", "", "app/models already declares NewReportResponse", func(t *testing.T) {
			writeFile(t, "app/models/report.go", "package models\n\nfunc NewReportResponse() {}\n")
		}},
		{"hand-written service", "Report", "", "app/services already declares ReportsService", func(t *testing.T) {
			writeFile(t, "app/services/reports_service.go", "package services\n\ntype ReportsService struct{}\n")
		}},
		{"hand-written controller", "Report", "", "app/controllers already declares ReportsController", func(t *testing.T) {
			writeFile(t, "app/controllers/reports_controller.go", "package controllers\n\ntype ReportsController struct{}\n")
		}},
		{"test function", "Report", "", "the controllers tests already declare TestReportsRequireAuth", func(t *testing.T) {
			writeFile(t, "app/controllers/reports_test.go", "package controllers_test\n\nfunc TestReportsRequireAuth() {}\n")
		}},
		{"predicate constant", "Outcome", "Course", "app/services already declares outcomesOwnedByUser", func(t *testing.T) {
			writeFile(t, "app/models/course.go", "package models\n\nimport \"github.com/uptrace/bun\"\n\ntype Course struct {\n\tbun.BaseModel `bun:\"table:courses,alias:courses\"`\n\n\tID     int64 `bun:\"id,pk,autoincrement\"`\n\tUserID int64 `bun:\"user_id,notnull\"`\n}\n")
			writeFile(t, "app/services/courses_service.go", "package services\n\ntype CoursesService struct{}\n\nfunc (s *CoursesService) VerifyOwnership(id, userID int64) error { return nil }\n\nconst outcomesOwnedByUser = \"\"\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyFixture(t)
			tt.setup(t)
			_, err := decideFor(t, tt.resource, nil, tt.parent)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}
