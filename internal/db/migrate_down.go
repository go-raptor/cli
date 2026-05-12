package db

import (
	"github.com/spf13/cobra"
)

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the last applied migration",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("down", "")
	},
}
