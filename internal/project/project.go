package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-raptor/cli/internal/configfiles"
)

func FindRoot() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		if hasConfigFile(dir) {
			return os.Chdir(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("not a Raptor project (raptor config file not found in any parent directory)")
		}
		dir = parent
	}
}

func ModuleName() (string, error) {
	content, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module name not found in go.mod")
}

func hasConfigFile(dir string) bool {
	for _, file := range configfiles.Runtime {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}
	return false
}
