package new

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const templateModule = "github.com/go-raptor/template"

//go:embed all:_template
var templateFS embed.FS

var Cmd = &cobra.Command{
	Use:   "new [module]",
	Short: "Create a new Raptor project",
	Long:  `Create a new Raptor project with the given Go module name.`,
	Args:  cobra.ExactArgs(1),
	Run:   newProject,
}

var spa bool

func init() {
	Cmd.Flags().BoolVar(&spa, "spa", false, "serve a single-page frontend from public/ with controllers/spa")
}

func newProject(cmd *cobra.Command, args []string) {
	moduleName := args[0]
	projectDir := moduleName[strings.LastIndex(moduleName, "/")+1:]

	if _, err := os.Stat(projectDir); err == nil {
		fmt.Printf("Directory %s already exists\n", projectDir)
		os.Exit(1)
	}

	fmt.Printf("Creating new 🦖 Raptor project: %s\n", projectDir)

	if err := createProject(projectDir, moduleName, templateData{SPA: spa}); err != nil {
		fmt.Printf("Error %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Project %s created successfully!\n\n", projectDir)
	fmt.Println("Get started:")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Println("  raptor dev")
	if spa {
		fmt.Println("\nFrontend: proxy /api to http://localhost:3000 in vite.config.ts, and copy its build into public/ for production (see README.md).")
	}
}

// createProject writes the template into projectDir with a go.mod, and resolves its
// dependencies.
func createProject(projectDir, moduleName string, data templateData) error {
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("creating project directory: %w", err)
	}

	if err := copyTemplateFiles(projectDir, moduleName, data); err != nil {
		return fmt.Errorf("creating project: %w", err)
	}

	if err := writeGoMod(projectDir, moduleName); err != nil {
		return fmt.Errorf("creating go.mod: %w", err)
	}

	fmt.Println("Resolving dependencies... 📦")
	if err := runInDir(projectDir, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}
	return nil
}

// templateData is what the *.tmpl files in _template are rendered with.
type templateData struct {
	SPA bool // serve a frontend build from public/ with controllers/spa
}

func copyTemplateFiles(projectDir, moduleName string, data templateData) error {
	return fs.WalkDir(templateFS, "_template", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, "_template")
		if relPath == "" {
			return nil
		}
		relPath = relPath[1:] // remove leading separator

		targetPath := filepath.Join(projectDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}

		// A .tmpl file differs between variants: render it, and drop the suffix
		if name, ok := strings.CutSuffix(targetPath, ".tmpl"); ok {
			if content, err = render(path, content, data); err != nil {
				return err
			}
			targetPath = name
		}

		// Replace template module name with the actual module name
		output := []byte(strings.ReplaceAll(string(content), templateModule, moduleName))

		// The new module path reorders imports, and a rendered conditional can leave a blank line
		if strings.HasSuffix(targetPath, ".go") {
			if output, err = format.Source(output); err != nil {
				return fmt.Errorf("formatting %s: %w", relPath, err)
			}
		}

		return os.WriteFile(targetPath, output, 0644)
	})
}

// render executes one .tmpl file of the template with data.
func render(name string, content []byte, data templateData) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeGoMod(projectDir, moduleName string) error {
	goVersion := runtime.Version()
	goVersion = strings.TrimPrefix(goVersion, "go")
	// Use only major.minor
	parts := strings.SplitN(goVersion, ".", 3)
	if len(parts) >= 2 {
		goVersion = parts[0] + "." + parts[1]
	}

	content := fmt.Sprintf("module %s\n\ngo %s\n", moduleName, goVersion)
	return os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(content), 0644)
}

func runInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
