package resource

import (
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

// TestEdgeCasesCompile generates, into one project, the models and names that once produced
// output that did not compile, and vets the result.
func TestEdgeCasesCompile(t *testing.T) {
	env := compileEnv(t)
	// Finding I-1: a parent owned through an embedded mixin, whose seed must set the promoted
	// user_id outside the composite literal.
	writeFile(t, "app/models/folder.go", folderModel)
	writeFile(t, "app/services/folders_service.go", foldersService)
	for _, o := range []Options{
		{Name: "Page", Fields: []string{"name:string"}, Parent: "Folder"},
	} {
		if err := run(t, o); err != nil {
			t.Fatalf("%s: %v", o.Name, err)
		}
	}
	vetProject(t, env)
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
