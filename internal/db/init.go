package db

import (
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-raptor/cli/internal/configfiles"
	"github.com/go-raptor/cli/internal/project"
	"github.com/spf13/cobra"
)

var initBun bool

var initCmd = &cobra.Command{
	Use:   "init <sqlite|postgres>",
	Short: "Add database support to the current Raptor project",
	Long: `Add database support to the current Raptor project.

Creates db/migrations.go and db/migrations/, registers the chosen connector in
config/components/components.go, adds a database section to your Raptor config
files, and runs go mod tidy.

Examples:
  raptor db init sqlite
  raptor db init sqlite --bun
  raptor db init postgres
  raptor db init postgres --bun`,
	Args: cobra.ExactArgs(1),
	Run:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initBun, "bun", false, "use the Bun ORM variant of the connector")
}

type connectorSpec struct {
	driver      string
	importPath  string
	selector    string
	constructor string
}

func connectorFor(driver string, bun bool) (connectorSpec, error) {
	switch driver {
	case "sqlite":
		path := "github.com/go-raptor/connectors/sqlite"
		if bun {
			path = "github.com/go-raptor/connectors/bun/sqlite"
		}
		return connectorSpec{driver: "sqlite", importPath: path, selector: "sqlite", constructor: "NewSQLiteConnector"}, nil
	case "postgres":
		if bun {
			return connectorSpec{driver: "postgres", importPath: "github.com/go-raptor/connectors/bun/postgres", selector: "postgres", constructor: "NewPostgresConnector"}, nil
		}
		return connectorSpec{driver: "postgres", importPath: "github.com/go-raptor/connectors/pgx", selector: "pgx", constructor: "NewPgxConnector"}, nil
	default:
		return connectorSpec{}, fmt.Errorf("unknown database driver %q (supported: sqlite, postgres)", driver)
	}
}

func databaseConfigBlock(spec connectorSpec, project string, test bool) string {
	switch spec.driver {
	case "sqlite":
		name := fmt.Sprintf("db/%s.db", project)
		if test {
			name = fmt.Sprintf("db/%s_test.db", project)
		}
		return fmt.Sprintf("database:\n  name: %q\n", name)
	case "postgres":
		name := project
		if test {
			name = project + "_test"
		}
		return fmt.Sprintf("database:\n  host: localhost\n  port: 5432\n  username: postgres\n  password: postgres\n  name: %s\n", name)
	default:
		return ""
	}
}

func addDatabaseConnector(src string, spec connectorSpec, moduleName string) (string, error) {
	const importMarker = "import ("
	importIdx := strings.Index(src, importMarker)
	if importIdx == -1 {
		return "", fmt.Errorf("could not find import block")
	}
	insertAt := importIdx + len(importMarker)
	imports := fmt.Sprintf("\n\t%q\n\t%q", spec.importPath, moduleName+"/db")
	src = src[:insertAt] + imports + src[insertAt:]

	const structMarker = "raptor.Components{"
	structIdx := strings.Index(src, structMarker)
	if structIdx == -1 {
		return "", fmt.Errorf("could not find raptor.Components{} literal")
	}
	fieldAt := structIdx + len(structMarker)
	field := fmt.Sprintf("\n\t\tDatabaseConnector: %s.%s(db.MigrationsFS()),", spec.selector, spec.constructor)
	src = src[:fieldAt] + field + src[fieldAt:]

	formatted, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("could not format result: %w", err)
	}
	return string(formatted), nil
}

const migrationsGoFile = `// Package db provides database access and management for the application, including handling database migrations.
package db

import (
	"embed"
	"io/fs"
)

//go:embed all:migrations
var migrationsFS embed.FS

func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}
`

func runInit(cmd *cobra.Command, args []string) {
	spec, err := connectorFor(strings.ToLower(args[0]), initBun)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := project.FindRoot(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	moduleName, err := project.ModuleName()
	if err != nil {
		fmt.Printf("Error reading go.mod: %v\n", err)
		os.Exit(1)
	}
	projectName := moduleName[strings.LastIndex(moduleName, "/")+1:]

	componentsPath := filepath.Join("config", "components", "components.go")
	componentsSrc, err := os.ReadFile(componentsPath)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", componentsPath, err)
		os.Exit(1)
	}

	// Idempotency: refuse to run twice.
	if _, err := os.Stat(filepath.Join("db", "migrations.go")); err == nil {
		fmt.Println("Database support already initialized (db/migrations.go exists).")
		os.Exit(1)
	}
	if strings.Contains(string(componentsSrc), "DatabaseConnector") {
		fmt.Println("Database support already initialized (DatabaseConnector already registered).")
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Join("db", "migrations"), 0o755); err != nil {
		fmt.Printf("Error creating db/migrations: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join("db", "migrations.go"), []byte(migrationsGoFile), 0o644); err != nil {
		fmt.Printf("Error writing db/migrations.go: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join("db", "migrations", ".keep"), nil, 0o644); err != nil {
		fmt.Printf("Error writing db/migrations/.keep: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created db/migrations.go and db/migrations/")

	// 2. Register the connector in components.go.
	updated, err := addDatabaseConnector(string(componentsSrc), spec, moduleName)
	if err != nil {
		fmt.Printf("Could not update %s automatically (%v).\n", componentsPath, err)
		fmt.Printf("Add import %q and field DatabaseConnector: %s.%s(db.MigrationsFS()) manually.\n",
			spec.importPath, spec.selector, spec.constructor)
	} else if err := os.WriteFile(componentsPath, []byte(updated), 0o644); err != nil {
		fmt.Printf("Error writing %s: %v\n", componentsPath, err)
		os.Exit(1)
	} else {
		fmt.Printf("Registered connector in %s\n", componentsPath)
	}

	for _, f := range configfiles.Runtime {
		appendDatabaseConfig(f, databaseConfigBlock(spec, projectName, false))
	}
	for _, f := range configfiles.Test {
		appendDatabaseConfig(f, databaseConfigBlock(spec, projectName, true))
	}

	fmt.Println("Resolving dependencies... 📦")
	if err := runGoModTidy(); err != nil {
		fmt.Printf("Error running go mod tidy: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Database support added!")
	fmt.Println("\nNext steps:")
	fmt.Println("  raptor db migrate create <name>   # create your first migration")
	fmt.Println("  raptor db migrate up              # apply migrations")
	fmt.Println("  raptor dev")
}

func appendDatabaseConfig(path, block string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return // file absent: nothing to do
	}
	s := string(content)
	if strings.HasPrefix(s, "database:") || strings.Contains(s, "\ndatabase:") {
		return // already configured
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "\n" + block
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		fmt.Printf("Error updating %s: %v\n", path, err)
		return
	}
	fmt.Printf("Configured database in %s\n", path)
}

func runGoModTidy() error {
	c := exec.Command("go", "mod", "tidy")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
