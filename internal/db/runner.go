package db

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-raptor/cli/internal/project"
)

// runInProject builds and runs the user's Raptor binary in the current
// project, asking it to perform a goose migration via the RAPTOR_MIGRATE_CMD
// env var. Go migrations registered through init() are only visible inside
// the compiled binary, so dispatching there (instead of from this CLI) is
// the only way to support them.
func runInProject(op string, version string) {
	if err := project.FindRoot(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	env := append(os.Environ(), "RAPTOR_MIGRATE_CMD="+op)
	if version != "" {
		env = append(env, "RAPTOR_MIGRATE_VERSION="+version)
	}

	c := exec.Command("go", "run", ".")
	c.Env = env
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		os.Exit(1)
	}
}
