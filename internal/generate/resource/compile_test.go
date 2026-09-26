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
