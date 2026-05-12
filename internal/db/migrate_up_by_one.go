package db

import (
	"github.com/spf13/cobra"
)

var migrateUpByOneCmd = &cobra.Command{
	Use:   "up-by-one",
	Short: "Apply the next pending migration",
	Run: func(cmd *cobra.Command, args []string) {
		runInProject("up-by-one", "")
	},
}
