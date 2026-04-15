package test

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/go-raptor/cli/internal/project"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "test [packages]",
	Short: "Run tests",
	Long: `Run tests for the Raptor project. Passes all arguments to go test.

Examples:
  raptor test
  raptor test ./app/controllers/...
  raptor test -v ./...
  raptor test -run TestGreet ./app/controllers/...`,
	DisableFlagParsing: true,
	Run:                runTests,
}

func runTests(cmd *cobra.Command, args []string) {
	if err := project.FindRoot(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	goArgs := []string{"test"}
	if len(args) == 0 {
		goArgs = append(goArgs, "./...")
	} else {
		goArgs = append(goArgs, args...)
	}

	c := exec.Command("go", goArgs...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		os.Exit(1)
	}
}
