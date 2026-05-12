package db

import (
	"github.com/spf13/cobra"
)

var migrateUpToCmd = &cobra.Command{
	Use:   "up-to <version>",
	Short: "Apply migrations up to and including the given version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("up-to", args[0])
	},
}
