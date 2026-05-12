package db

import (
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long: `Run database migrations using goose.

Examples:
  raptor db migrate up
  raptor db migrate up-by-one
  raptor db migrate up-to 20250806224701
  raptor db migrate down
  raptor db migrate down-to 20250806224701
  raptor db migrate redo
  raptor db migrate reset
  raptor db migrate status
  raptor db migrate version
  raptor db migrate create add_users [--sql|--go]`,
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateUpByOneCmd)
	migrateCmd.AddCommand(migrateUpToCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateDownToCmd)
	migrateCmd.AddCommand(migrateRedoCmd)
	migrateCmd.AddCommand(migrateResetCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
	migrateCmd.AddCommand(migrateCreateCmd)
}
