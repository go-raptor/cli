package new

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestNewProjectKeepsBodyLimit(t *testing.T) {
	dir := t.TempDir()
	if err := copyTemplateFiles(dir, "example.com/shop"); err != nil {
		t.Fatal(err)
	}
	disabled := regexp.MustCompile(`(?m)^\s*max_body_bytes:\s*0\s*$`)
	for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if disabled.Match(b) {
			t.Errorf("%s disables the request body limit:\n%s", name, b)
		}
	}
}
