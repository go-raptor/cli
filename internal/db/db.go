package db

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "db",
	Short: "Database operations",
	Long:  `Database operations: migrations, etc.`,
}

func init() {
	Cmd.AddCommand(migrateCmd)
}
