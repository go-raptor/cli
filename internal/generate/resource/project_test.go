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
