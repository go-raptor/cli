package db

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var migrateDownToCmd = &cobra.Command{
	Use:   "down-to <version>",
	Short: "Roll back migrations down to (but not including) the given version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		if version == "0" {
			if !confirm("This will roll back ALL applied migrations. Continue? [y/N]: ") {
				fmt.Println("Aborted.")
				os.Exit(0)
			}
		}
		runInProject("down-to", version)
	},
}
