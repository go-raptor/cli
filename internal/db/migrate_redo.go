package db

import (
	"github.com/spf13/cobra"
)

var migrateRedoCmd = &cobra.Command{
	Use:   "redo",
	Short: "Roll back and re-apply the last migration",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("redo", "")
	},
}
