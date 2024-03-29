package cmd

import (
	"os"

	"github.com/go-raptor/cli/internal/dev"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "raptor",
	Short: "Go MVC web development eco-system based on Fiber",
}

func Execute() {
	rootCmd.AddCommand(dev.Cmd)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
