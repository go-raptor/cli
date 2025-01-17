package version

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	version = "v1.0.4"
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Raptor CLI",
	Run:   versionCmd,
}

func versionCmd(cmd *cobra.Command, args []string) {
	fmt.Printf("Raptor CLI %s\n", version)
}
