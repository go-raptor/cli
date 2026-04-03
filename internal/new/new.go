package new

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

func newProject(cmd *cobra.Command, args []string) {
	moduleName := args[0]
	projectDir := moduleName[strings.LastIndex(moduleName, "/")+1:]

	if _, err := os.Stat(projectDir); err == nil {
		fmt.Printf("Directory %s already exists\n", projectDir)
		os.Exit(1)
	}

	fmt.Printf("Creating new 🦖 Raptor project: %s\n", projectDir)

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Printf("Error creating project directory: %v\n", err)
		os.Exit(1)
	}

	if err := copyTemplateFiles(projectDir, moduleName); err != nil {
		fmt.Printf("Error creating project: %v\n", err)
		os.Exit(1)
	}

	if err := writeGoMod(projectDir, moduleName); err != nil {
		fmt.Printf("Error creating go.mod: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Resolving dependencies... 📦")
	if err := runInDir(projectDir, "go", "mod", "tidy"); err != nil {
		fmt.Printf("Error resolving dependencies: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Project %s created successfully!\n\n", projectDir)
	fmt.Println("Get started:")
	fmt.Printf("  cd %s\n", projectDir)
	fmt.Println("  raptor dev")
}

func copyTemplateFiles(projectDir, moduleName string) error {
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

		// Replace template module name with the actual module name
		output := strings.ReplaceAll(string(content), templateModule, moduleName)

		return os.WriteFile(targetPath, []byte(output), 0644)
	})
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
