package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-raptor/cli/internal/components"
	"github.com/go-raptor/cli/internal/naming"
	"github.com/go-raptor/cli/internal/project"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "generate [type] [name]",
	Aliases: []string{"g"},
	Short:   "Generate a new component",
	Long: `Generate a new controller, service, middleware, or model.

Examples:
  raptor generate controller Users
  raptor g service Auth
  raptor g middleware RateLimit
  raptor g model User`,
	Args: cobra.ExactArgs(2),
	Run:  generate,
}

var typeSuffixes = map[string]string{
	"controller": "Controller",
	"service":    "Service",
	"middleware": "Middleware",
}

func generate(cmd *cobra.Command, args []string) {
	componentType := strings.ToLower(args[0])
	name := args[1]

	switch componentType {
	case "controller", "service", "middleware", "model":
	default:
		fmt.Printf("Unknown component type: %s\n", componentType)
		fmt.Println("Available types: controller, service, middleware, model")
		os.Exit(1)
	}

	if err := project.FindRoot(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	moduleName, err := getModuleName()
	if err != nil {
		fmt.Printf("Error reading go.mod: %v\n", err)
		os.Exit(1)
	}

	// Normalize the name: strip type suffix if present, convert to snake_case, then PascalCase
	if suffix, ok := typeSuffixes[componentType]; ok {
		name = strings.TrimSuffix(name, suffix)
	}
	snakeName := naming.Snake(name)
	pascalName := naming.Pascal(snakeName)

	switch componentType {
	case "controller":
		generateController(moduleName, snakeName, pascalName)
	case "service":
		generateService(moduleName, snakeName, pascalName)
	case "middleware":
		generateMiddleware(snakeName, pascalName)
	case "model":
		generateModel(snakeName, pascalName)
	}
}

func generateController(moduleName, snakeName, pascalName string) {
	dir := filepath.Join("app", "controllers")
	fileName := snakeName + "_controller.go"
	structName := pascalName + "Controller"

	content := fmt.Sprintf(`package controllers

import (
	"github.com/go-raptor/raptor/v4"
)

type %s struct {
	raptor.Controller
}
`, structName)

	if err := writeComponent(dir, fileName, content); err != nil {
		fmt.Printf("Error creating controller: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, fileName))

	if err := components.Register(moduleName, "controller", structName); err != nil {
		fmt.Printf("Register %s in config/components/controllers.go\n", structName)
	} else {
		fmt.Printf("Registered %s in config/components/controllers.go\n", structName)
	}

	generateControllerTest(moduleName, dir, snakeName, structName)
}

func generateService(moduleName, snakeName, pascalName string) {
	dir := filepath.Join("app", "services")
	fileName := snakeName + "_service.go"
	structName := pascalName + "Service"

	content := fmt.Sprintf(`package services

import (
	"github.com/go-raptor/raptor/v4"
)

type %s struct {
	raptor.Service
}

func (s *%s) Setup() error {
	return nil
}

func (s *%s) Cleanup() error {
	return nil
}
`, structName, structName, structName)

	if err := writeComponent(dir, fileName, content); err != nil {
		fmt.Printf("Error creating service: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, fileName))

	if err := components.Register(moduleName, "service", structName); err != nil {
		fmt.Printf("Register %s in config/components/services.go\n", structName)
	} else {
		fmt.Printf("Registered %s in config/components/services.go\n", structName)
	}

	generateServiceTest(moduleName, dir, snakeName, structName)
}

func generateMiddleware(snakeName, pascalName string) {
	dir := filepath.Join("app", "middlewares")
	fileName := snakeName + "_middleware.go"
	structName := pascalName + "Middleware"

	content := fmt.Sprintf(`package middlewares

import (
	"github.com/go-raptor/raptor/v4"
)

type %s struct {
	raptor.Middleware
}

func (m *%s) Setup() error {
	return nil
}

func (m *%s) Handle(ctx *raptor.Context, next func(*raptor.Context) error) error {
	return next(ctx)
}
`, structName, structName, structName)

	if err := writeComponent(dir, fileName, content); err != nil {
		fmt.Printf("Error creating middleware: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, fileName))
	fmt.Printf("Register %s in config/components/middlewares.go\n", structName)
}

func generateModel(snakeName, pascalName string) {
	dir := filepath.Join("app", "models")
	fileName := snakeName + ".go"

	content := fmt.Sprintf(`package models

type %s struct {
	ID int64 `+"`json:\"id\"`"+`
}
`, pascalName)

	if err := writeComponent(dir, fileName, content); err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, fileName))
}

func writeComponent(dir, fileName, content string) error {
	filePath := filepath.Join(dir, fileName)

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("%s already exists", filePath)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

func getModuleName() (string, error) {
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

func ensureTestSetup(moduleName, dir, packageName string) {
	setupPath := filepath.Join(dir, "setup_test.go")
	if _, err := os.Stat(setupPath); err == nil {
		return
	}

	content := fmt.Sprintf(`package %s_test

import (
	"os"
	"testing"

	"github.com/go-raptor/raptor/v4"
	"%s/config"
	"%s/config/components"
)

var app *raptor.Raptor

func TestMain(m *testing.M) {
	app = raptor.NewTestApp(components.New(), config.Routes())
	os.Exit(m.Run())
}
`, packageName, moduleName, moduleName)

	if err := os.WriteFile(setupPath, []byte(content), 0644); err != nil {
		fmt.Printf("Error creating %s: %v\n", setupPath, err)
		return
	}
	fmt.Printf("Created %s\n", setupPath)
}

func generateControllerTest(moduleName, dir, snakeName, structName string) {
	ensureTestSetup(moduleName, dir, "controllers")

	testFileName := snakeName + "_controller_test.go"
	testContent := fmt.Sprintf(`package controllers_test
`)

	if err := writeComponent(dir, testFileName, testContent); err != nil {
		fmt.Printf("Error creating controller test: %v\n", err)
		return
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, testFileName))
}

func generateServiceTest(moduleName, dir, snakeName, structName string) {
	ensureTestSetup(moduleName, dir, "services")

	testFileName := snakeName + "_service_test.go"
	testContent := fmt.Sprintf(`package services_test
`)

	if err := writeComponent(dir, testFileName, testContent); err != nil {
		fmt.Printf("Error creating service test: %v\n", err)
		return
	}
	fmt.Printf("Created %s\n", filepath.Join(dir, testFileName))
}
