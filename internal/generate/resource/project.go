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
	"time"
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

// declares reports whether the package declares name at the top level.
func (d decls) declares(name string) bool { return d.Funcs[name] || d.Vars[name] || d.Types[name] }

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
	Routes          string    // config/routes.yaml; "" when missing
	Migrations      string    // every migration file, lowercased
	LatestMigration time.Time // the newest timestamp version in db/migrations; zero without one
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
			version, _, _ := strings.Cut(e.Name(), "_")
			if at, err := time.ParseInLocation(migrationStamp, version, time.UTC); err == nil && at.After(p.LatestMigration) {
				p.LatestMigration = at
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
	problems = append(problems, p.nameClashes(res, v)...)
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

// nameClashes lists the package-level names the new files would declare that their package
// already declares, Bun model or not.
func (p *Project) nameClashes(res *Resource, v *view) []string {
	var out []string
	clash := func(d decls, where, name string) {
		if d.declares(name) {
			out = append(out, where+" "+name)
		}
	}
	for _, d := range res.modelDecls() {
		clash(p.ModelsPkg, "app/models already declares", d.name)
	}
	clash(p.Services, "app/services already declares", v.Plural+"Service")
	if v.PredicateConst != "" {
		clash(p.Services, "app/services already declares", v.PredicateConst)
	}
	clash(p.Controllers, "app/controllers already declares", v.Plural+"Controller")
	for _, fn := range v.testFuncs() {
		clash(p.ControllerTests, "the controllers tests already declare", fn)
	}
	return out
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
			field, ok := p.Models["User"].Field(f)
			if !ok {
				return "models.User has no " + f + " field for the generated test harness"
			}
			if field.Embed != "" { // the harness sets it in a composite literal
				return fmt.Sprintf("models.User's %s comes from the embedded %s, which the generated harness's composite literal cannot set", f, field.Embed)
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
