package new

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// appConfig is the app: mapping of a generated config file.
func appConfig(t *testing.T, dir, name string) map[string]string {
	t.Helper()
	var cfg struct {
		App map[string]string `yaml:"app"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, dir, name)), &cfg); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return cfg.App
}

// Neither variant opens the API to other origins: csrf protects writes, and nothing registers
// cors or lists cors_allow_origins.
func TestNoVariantIsCrossOrigin(t *testing.T) {
	for variant, dir := range variants(t) {
		mw := readFile(t, dir, "config/components/middlewares.go")
		if !strings.Contains(mw, "raptor.Use(&csrf.CSRFMiddleware{})") {
			t.Errorf("%s: middlewares.go does not register csrf:\n%s", variant, mw)
		}
		if strings.Contains(mw, "middlewares/cors") {
			t.Errorf("%s: middlewares.go registers cors:\n%s", variant, mw)
		}
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if _, ok := appConfig(t, dir, name)["cors_allow_origins"]; ok {
				t.Errorf("%s: %s sets cors_allow_origins", variant, name)
			}
		}
	}
}

// routes is config/routes.yaml as the router reads it.
func routes(t *testing.T, dir string) map[string]any {
	t.Helper()
	var r struct {
		Routes map[string]any `yaml:"routes"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, dir, "config/routes.yaml")), &r); err != nil {
		t.Fatal(err)
	}
	return r.Routes
}

// --spa wires controllers/spa exactly as its README routes it, relaxes it only where there is no
// build (development and tests), and keeps the build out of git. The API-only default has none
// of it.
func TestSPAVariantServesTheFrontend(t *testing.T) {
	dirs := variants(t)
	for variant, want := range map[string]bool{"api": false, "spa": true} {
		dir := dirs[variant]
		controllers := readFile(t, dir, "config/components/controllers.go")
		if got := strings.Contains(controllers, "&spa.SPAController{}"); got != want {
			t.Errorf("%s: registers SPAController = %v, want %v:\n%s", variant, got, want, controllers)
		}
		r := routes(t, dir)
		api, _ := r["/api/v1"].(map[string]any)
		if got := r["/"] == "SPA.Index" && api["/{path...}"] == "Errors.NotFound"; got != want {
			t.Errorf("%s: routes / to SPA.Index and keeps the JSON 404 for /api/v1 = %v, want %v: %v", variant, got, want, r)
		}
		if api["/hello"] == nil {
			t.Errorf("%s: lost the /api/v1/hello routes: %v", variant, r)
		}
		for _, name := range []string{".raptor.dev.yaml", ".raptor.test.yaml"} {
			if got := appConfig(t, dir, name)["spa_optional"] == "true"; got != want {
				t.Errorf("%s: %s sets spa_optional \"true\" = %v, want %v", variant, name, got, want)
			}
		}
		if got := slices.Contains(strings.Split(readFile(t, dir, ".gitignore"), "\n"), "/public/"); got != want {
			t.Errorf("%s: .gitignore ignores /public/ = %v, want %v", variant, got, want)
		}
	}
}

func TestSPAIsAnOptInFlag(t *testing.T) {
	if f := Cmd.Flags().Lookup("spa"); f == nil || f.DefValue != "false" {
		t.Fatalf("--spa must be a flag that defaults to off, got %+v", f)
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
