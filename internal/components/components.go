// Package components registers controllers and services in config/components.
package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddEntry adds &<pkg>.<structName>{} to the Controllers or Services slice in src, importing
// the package if needed. An already registered component leaves src unchanged.
func AddEntry(src, moduleName, kind, structName string) (string, error) {
	var importPkg, marker, entry string
	switch kind {
	case "controller":
		importPkg = moduleName + "/app/controllers"
		marker = "raptor.Controllers{"
		entry = fmt.Sprintf("&controllers.%s{},", structName)
	case "service":
		importPkg = moduleName + "/app/services"
		marker = "raptor.Services{"
		entry = fmt.Sprintf("&services.%s{},", structName)
	default:
		return "", fmt.Errorf("unsupported component type for registration: %s", kind)
	}

	s := src
	if strings.Contains(s, structName) {
		return s, nil
	}

	if !strings.Contains(s, importPkg) {
		raptorImport := "\"github.com/go-raptor/raptor/v4\""
		idx := strings.Index(s, raptorImport)
		if idx == -1 {
			return "", fmt.Errorf("could not find the raptor import")
		}
		lineEnd := idx + len(raptorImport)
		s = s[:lineEnd] + fmt.Sprintf("\n\t\"%s\"", importPkg) + s[lineEnd:]
	}

	markerIdx := strings.Index(s, marker)
	if markerIdx == -1 {
		return "", fmt.Errorf("could not find %s", marker)
	}
	depth := 0
	closingIdx := -1
	for i := markerIdx + strings.Index(marker, "{"); i < len(s); i++ {
		if s[i] == '{' {
			depth++
		}
		if s[i] == '}' {
			depth--
			if depth == 0 {
				closingIdx = i
				break
			}
		}
	}
	if closingIdx == -1 {
		return "", fmt.Errorf("could not find the closing brace of %s", marker)
	}
	nlIdx := strings.LastIndex(s[:closingIdx], "\n")
	return s[:nlIdx+1] + fmt.Sprintf("\t\t%s\n", entry) + s[nlIdx+1:], nil
}

// Register adds structName to config/components/<kind>s.go.
func Register(moduleName, kind, structName string) error {
	regFile := filepath.Join("config", "components", kind+"s.go")
	content, err := os.ReadFile(regFile)
	if err != nil {
		return err
	}
	out, err := AddEntry(string(content), moduleName, kind, structName)
	if err != nil {
		return fmt.Errorf("%s: %w", regFile, err)
	}
	if out == string(content) {
		return nil
	}
	return os.WriteFile(regFile, []byte(out), 0644)
}
