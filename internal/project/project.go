package project

import (
	"fmt"
	"os"
	"path/filepath"
)

var configFiles = []string{
	".raptor.yaml",
	".raptor.yml",
	".raptor.conf",
	".raptor.prod.yaml",
	".raptor.prod.yml",
	".raptor.prod.conf",
	".raptor.dev.yaml",
	".raptor.dev.yml",
	".raptor.dev.conf",
}

// FindRoot walks up the directory tree from the current working directory
// looking for a Raptor config file. If found, it changes the working directory
// to the project root and returns nil. If not found, it returns an error.
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

func hasConfigFile(dir string) bool {
	for _, file := range configFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}
	return false
}
