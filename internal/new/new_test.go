package new

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// variants renders the template into a temporary directory per variant, keyed "api" and "spa".
func variants(t *testing.T) map[string]string {
	t.Helper()
	dirs := map[string]string{}
	for name, data := range map[string]templateData{"api": {}, "spa": {SPA: true}} {
		dir := t.TempDir()
		if err := copyTemplateFiles(dir, "example.com/shop", data); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		dirs[name] = dir
	}
	return dirs
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// rendered lists the files the .tmpl sources become, dotfiles included.
var rendered = []string{".gitignore", ".raptor.dev.yaml", ".raptor.test.yaml", "README.md", "config/routes.yaml", "config/components/controllers.go"}

// Both variants render completely: every .tmpl source becomes its file, no template action or
// placeholder module path is left, the YAML parses, and Go comes out gofmt-clean.
func TestTemplateRendersInBothVariants(t *testing.T) {
	for variant, dir := range variants(t) {
		for _, name := range rendered {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
				t.Errorf("%s: %s is missing", variant, name)
			}
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.HasSuffix(rel, ".tmpl") {
				t.Errorf("%s: %s was copied instead of rendered", variant, rel)
			}
			if bytes.Contains(b, []byte("{{")) {
				t.Errorf("%s: %s still holds a template action", variant, rel)
			}
			if bytes.Contains(b, []byte(templateModule)) {
				t.Errorf("%s: %s still names %s", variant, rel, templateModule)
			}
			if strings.HasSuffix(rel, ".yaml") {
				var v map[string]any
				if err := yaml.Unmarshal(b, &v); err != nil {
					t.Errorf("%s: %s does not parse: %v", variant, rel, err)
				}
			}
			if strings.HasSuffix(rel, ".go") {
				if formatted, err := format.Source(b); err != nil || !bytes.Equal(formatted, b) {
					t.Errorf("%s: %s is not gofmt-clean (%v)", variant, rel, err)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewProjectKeepsBodyLimit(t *testing.T) {
	disabled := regexp.MustCompile(`(?m)^\s*max_body_bytes:\s*0\s*$`)
	for variant, dir := range variants(t) {
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if b := readFile(t, dir, name); disabled.MatchString(b) {
				t.Errorf("%s: %s disables the request body limit:\n%s", variant, name, b)
			}
		}
	}
}
