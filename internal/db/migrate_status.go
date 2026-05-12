package db

import (
	"github.com/spf13/cobra"
)

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "List all migrations and their state",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("status", "")
	},
}
