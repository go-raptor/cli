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
			"app/services/folders_service.go": ownershipService("FoldersService"),
			"app/models/box.go":               boxModels,
			"app/services/boxes_service.go":   ownershipService("BoxesService"),
			"app/services/lockers_service.go": ownershipService("LockersService"),
		}, []Options{
			// Finding I-1: a parent owned through an embedded mixin, whose seed must set the
			// promoted user_id outside the composite literal.
			{Name: "Page", Fields: []string{"name:string"}, Parent: "Folder"},
			// Fix round 2, N-1: parents owned through an unexported nested mixin and through a
			// named embed field; their seeds set user_id through the selector Go allows.
			{Name: "Lid", Fields: []string{"name:string"}, Parent: "Box"},
			{Name: "Latch", Fields: []string{"name:string"}, Parent: "Locker"},
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
			"app/models/room.go":    roomModel,
		}, []Options{
			// Finding I-3: the seeds generated for existing models named Context and Db must not
			// shadow the context package or the db helper.
			{Name: "Shift", Fields: []string{"name:string", "context:ref", "db:ref"}},
			// Fix round 2, N-1: reference data whose Code comes from a lowercase mixin.
			{Name: "Meeting", Fields: []string{"title:string", "room:ref"}},
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

// ownershipService is a hand-written parent service: all a child's service needs of it.
func ownershipService(name string) string {
	return "package services\n\nimport \"github.com/go-raptor/raptor/v4\"\n\ntype " + name + " struct {\n\traptor.Service\n}\n\nfunc (s *" + name + ") VerifyOwnership(id, userID int64) error { return nil }\n"
}

const boxModels = `package models

import "github.com/uptrace/bun"

type ownedInner struct {
	UserID int64 ` + "`" + `bun:"user_id,notnull" json:"-"` + "`" + `
}

type Mid struct {
	ownedInner
}

// Box is owned through two levels of embedding, one of them unexported.
type Box struct {
	bun.BaseModel ` + "`" + `bun:"table:boxes,alias:boxes"` + "`" + `

	ID int64 ` + "`" + `bun:"id,pk,autoincrement" json:"id"` + "`" + `
	Mid
}

// Locker holds its owner in a named embed field, which Go doesn't promote.
type Locker struct {
	bun.BaseModel ` + "`" + `bun:"table:lockers,alias:lockers"` + "`" + `

	ID  int64 ` + "`" + `bun:"id,pk,autoincrement" json:"id"` + "`" + `
	Own Owned ` + "`" + `bun:"embed:"` + "`" + `
}
`

const roomModel = `package models

import "github.com/uptrace/bun"

type coded struct {
	Code string ` + "`" + `bun:"code,notnull,unique" json:"code"` + "`" + `
}

// Room is shared reference data with a lowercase mixin.
type Room struct {
	bun.BaseModel ` + "`" + `bun:"table:rooms,alias:rooms"` + "`" + `

	ID int64 ` + "`" + `bun:"id,pk,autoincrement" json:"id"` + "`" + `
	coded
	Name string ` + "`" + `bun:"name,notnull" json:"name"` + "`" + `
}
`
