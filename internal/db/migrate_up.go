package db

import (
	"github.com/spf13/cobra"
)

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("up", "")
	},
}
