package resource

import (
	"fmt"
	"go/build"
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
// Unchanged is printed when Apply returns the file as it was.
type Edit struct {
	Path      string
	Apply     func(src string) (string, error)
	Manual    string
	Unchanged string
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
	modelFile := snake + ".go"
	if goSkips(modelFile) {
		modelFile = snake + "_model.go"
	}
	for _, s := range []struct {
		ok         bool
		path, tmpl string
	}{
		{true, filepath.Join("app", "models", modelFile), "model.go.tmpl"},
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
	// The table migration is stamped now, and the setup migration a second earlier, so Goose
	// applies it first. Both go after the newest existing version, so runs chained within one
	// second never share one.
	stamp := now.UTC().Truncate(time.Second)
	if d.SetupMigration {
		stamp = stamp.Add(-time.Second)
	}
	if next := p.LatestMigration.Add(time.Second); stamp.Before(next) {
		stamp = next
	}
	if d.SetupMigration {
		g.Files = append(g.Files, File{filepath.Join("db", "migrations", stamp.Format(migrationStamp)+"_setup.sql"), setupMigration})
		stamp = stamp.Add(time.Second)
	}
	g.Files = append(g.Files, File{filepath.Join("db", "migrations", stamp.Format(migrationStamp)+"_create_"+v.Table+".sql"), renderMigration(v)})
	written := map[string]bool{}
	for _, f := range g.Files {
		if written[f.Path] {
			return nil, fmt.Errorf("%s would be written twice (two generated files share the name); choose another name", f.Path)
		}
		written[f.Path] = true
	}

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

// goSkips reports whether the go command leaves a file of this name out of the package: a
// _test.go file, or one whose name limits it to an OS or architecture (x_windows.go).
func goSkips(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return true
	}
	for _, target := range [][2]string{{"linux", "amd64"}, {"windows", "arm64"}} {
		ctxt := build.Default
		ctxt.GOOS, ctxt.GOARCH = target[0], target[1]
		ctxt.OpenFile = func(string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("package models\n")), nil
		}
		if ok, err := ctxt.MatchFile(".", name); err != nil || !ok {
			return true
		}
	}
	return false
}

func registerEdit(module, kind, structName string) Edit {
	list := "raptor.Services{…}"
	if kind == "controller" {
		list = "raptor.Controllers{…}"
	}
	path := filepath.Join("config", "components", kind+"s.go")
	return Edit{
		Path:      path,
		Apply:     func(src string) (string, error) { return components.AddEntry(src, module, kind, structName) },
		Manual:    fmt.Sprintf("add &%ss.%s{} to %s", kind, structName, list),
		Unchanged: fmt.Sprintf("%s already registers %s", path, structName),
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
		if err == nil && out == string(src) {
			if e.Unchanged == "" {
				e.Unchanged = "Unchanged " + e.Path
			}
			fmt.Fprintln(o.Out, e.Unchanged)
			continue
		}
		if err == nil {
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
