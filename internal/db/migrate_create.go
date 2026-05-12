package db

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/go-raptor/cli/internal/project"
	"github.com/spf13/cobra"
)

//go:embed templates/sql_migration.tmpl templates/go_migration.tmpl
var templateFS embed.FS

var (
	createSQL bool
	createGo  bool
)

var migrateCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new migration file in db/migrations/",
	Long: `Create a new migration file. Defaults to SQL format.

Examples:
  raptor db migrate create add_users
  raptor db migrate create add_users --sql
  raptor db migrate create populate_settings --go`,
	Args: cobra.ExactArgs(1),
	Run:  createMigration,
}

func init() {
	migrateCreateCmd.Flags().BoolVar(&createSQL, "sql", false, "create a SQL migration (default)")
	migrateCreateCmd.Flags().BoolVar(&createGo, "go", false, "create a Go migration")
	migrateCreateCmd.MarkFlagsMutuallyExclusive("sql", "go")
}

func createMigration(cmd *cobra.Command, args []string) {
	if err := project.FindRoot(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	name := toSnakeCase(args[0])
	if name == "" {
		fmt.Println("Migration name cannot be empty")
		os.Exit(1)
	}

	ext := "sql"
	if createGo {
		ext = "go"
	}

	dir := filepath.Join("db", "migrations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Failed to create %s: %v\n", dir, err)
		os.Exit(1)
	}

	timestamp := time.Now().UTC().Format("20060102150405")
	filename := fmt.Sprintf("%s_%s.%s", timestamp, name, ext)
	target := filepath.Join(dir, filename)

	if _, err := os.Stat(target); err == nil {
		fmt.Printf("File already exists: %s\n", target)
		os.Exit(1)
	}

	content, err := renderTemplate(ext, name)
	if err != nil {
		fmt.Printf("Failed to render template: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", target, err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", target)
}

func renderTemplate(ext, snakeName string) (string, error) {
	var tmplPath string
	switch ext {
	case "sql":
		tmplPath = "templates/sql_migration.tmpl"
	case "go":
		tmplPath = "templates/go_migration.tmpl"
	default:
		return "", fmt.Errorf("unsupported extension %q", ext)
	}

	raw, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(tmplPath).Parse(string(raw))
	if err != nil {
		return "", err
	}

	var out strings.Builder
	data := struct{ Pascal string }{Pascal: toPascalCase(snakeName)}
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}
