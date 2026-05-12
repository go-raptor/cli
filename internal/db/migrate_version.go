package db

import (
	"github.com/spf13/cobra"
)

var migrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the current migration version",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("version", "")
	},
}
