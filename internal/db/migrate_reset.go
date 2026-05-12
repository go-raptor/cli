package db

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Roll back all applied migrations",
	Run: func(cmd *cobra.Command, args []string) {
		if !confirm("This will roll back ALL applied migrations. Continue? [y/N]: ") {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
		runInProject("reset", "")
	},
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}
