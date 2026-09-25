package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInitPostgresKeepsPasswordOutOfConfig(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("server:\n  port: 3000\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)

	spec, err := connectorFor("postgres", false)
	if err != nil {
		t.Fatal(err)
	}
	configureDatabase(spec, "shop")

	passwordKey := regexp.MustCompile(`(?m)^\s*password:`)
	for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "\ndatabase:") {
			t.Fatalf("%s: database section not written:\n%s", name, b)
		}
		if passwordKey.Match(b) {
			t.Errorf("%s puts a password in a tracked file:\n%s", name, b)
		}
		if !strings.Contains(string(b), "DATABASE_PASSWORD") {
			t.Errorf("%s should say where the password comes from:\n%s", name, b)
		}
	}
}
