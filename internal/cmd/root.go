package cmd

import (
	"os"

	"github.com/go-raptor/cli/internal/dev"
	"github.com/go-raptor/cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "raptor",
	Short: "Go MVCS web development eco-system based on Echo",
}

func Execute() {
	rootCmd.AddCommand(dev.Cmd)
	rootCmd.AddCommand(version.Cmd)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
