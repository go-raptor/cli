package resource

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// compileEnv copies the fixture project for go vet, or skips the test under -short or when the
// fixture's modules can't be downloaded.
func compileEnv(t *testing.T) []string {
	t.Helper()
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
	return env
}

func vetProject(t *testing.T, env []string) {
	t.Helper()
	vet := exec.Command("go", "vet", "./...")
	vet.Env = env
	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("go vet on the generated project: %v\n%s", err, out)
	}
}

// TestGeneratedProjectCompiles generates all four scenarios into the fixture project and vets
// it, which type-checks every generated file, tests included, against the real Raptor, Bun and
// zog APIs.
func TestGeneratedProjectCompiles(t *testing.T) {
	env := compileEnv(t)
	runScenarios(t)
	vetProject(t, env)
}

// TestEdgeCasesCompile generates the models and names that once produced output that did not
// compile, one project per group, and vets each project.
func TestEdgeCasesCompile(t *testing.T) {
	for _, project := range []struct {
		name  string
		files map[string]string
		runs  []Options
	}{
		{"locals", map[string]string{
			"app/models/folder.go":            folderModel,
			"app/services/folders_service.go": foldersService,
		}, []Options{
			// Finding I-1: a parent owned through an embedded mixin, whose seed must set the
			// promoted user_id outside the composite literal.
			{Name: "Page", Fields: []string{"name:string"}, Parent: "Folder"},
			// Finding I-3: names whose variables would shadow a package, a harness helper or a
			// test local get renamed locals.
			{Name: "Model", Fields: []string{"name:string"}},
			{Name: "Context", Fields: []string{"name:string"}},
			{Name: "Login", Fields: []string{"name:string"}},
			{Name: "Db", Fields: []string{"name:string"}},
			{Name: "Current", Fields: []string{"name:string"}, Parent: "Model"},
			{Name: "OtherModel", Fields: []string{"name:string"}, Parent: "Model"},
			{Name: "StrangersModel", Fields: []string{"name:string"}, Parent: "Model", Movable: true},
			// Finding I-3: a model file named lab_test.go would be a test file.
			{Name: "LabTest", Fields: []string{"name:string"}},
		}},
		{"seeds", map[string]string{
			"app/models/context.go": referenceModel("Context", "contexts"),
			"app/models/db.go":      referenceModel("Db", "dbs"),
		}, []Options{
			// Finding I-3: the seeds generated for existing models named Context and Db must not
			// shadow the context package or the db helper.
			{Name: "Shift", Fields: []string{"name:string", "context:ref", "db:ref"}},
		}},
	} {
		t.Run(project.name, func(t *testing.T) {
			env := compileEnv(t)
			for path, content := range project.files {
				writeFile(t, path, content)
			}
			for _, o := range project.runs {
				if err := run(t, o); err != nil {
					t.Fatalf("%s: %v", o.Name, err)
				}
			}
			vetProject(t, env)
		})
	}
}

func referenceModel(name, table string) string {
	return fmt.Sprintf("package models\n\nimport \"github.com/uptrace/bun\"\n\ntype %s struct {\n\tbun.BaseModel `bun:\"table:%s,alias:%s\"`\n\n\tID    int64  `bun:\"id,pk,autoincrement\" json:\"id\"`\n\tLabel string `bun:\"label,notnull\" json:\"label\"`\n}\n", name, table, table)
}

const folderModel = `package models

import "github.com/uptrace/bun"

// Owned is a mixin: its user_id lands on the table of the model that embeds it.
type Owned struct {
	UserID int64 ` + "`" + `bun:"user_id,notnull" json:"-"` + "`" + `
	User   *User ` + "`" + `bun:"rel:belongs-to,join:user_id=id" json:"-"` + "`" + `
}

type Folder struct {
	bun.BaseModel ` + "`" + `bun:"table:folders,alias:folders"` + "`" + `

	ID int64 ` + "`" + `bun:"id,pk,autoincrement" json:"id"` + "`" + `
	Owned
	Name string ` + "`" + `bun:"name,notnull" json:"name"` + "`" + `
}
`

const foldersService = `package services

import "github.com/go-raptor/raptor/v4"

type FoldersService struct {
	raptor.Service
}

func (s *FoldersService) VerifyOwnership(id, userID int64) error { return nil }
`
