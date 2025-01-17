package dev

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const (
	colorGreen = "\033[0;32m"
	colorRed   = "\033[0;31m"
	colorNone  = "\033[0m"
)

var (
	watcher           *fsnotify.Watcher
	ignoreDirectories []string
	runningCmd        *exec.Cmd
)

var Cmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long:  `Start the development server with hot reload.`,
	Run:   developmentServer,
}

var defaultIgnoreDirectories = []string{
	"bin",
	".git",
	"tmp",
	"vendor",
}

var configFiles = []string{
	".raptor.conf",
	".raptor.toml",
	".raptor.dev.conf",
	".raptor.dev.toml",
	".raptor.prod.conf",
	".raptor.prod.toml",
}

func developmentServer(cmd *cobra.Command, args []string) {
	var err error
	for _, file := range configFiles {
		_, err = os.Stat(file)
		if err == nil {
			break
		}
	}

	if err != nil {
		fmt.Println("Please run this command in the root of Raptor project")
		os.Exit(1)
	}

	fmt.Println("Starting 🦖 Raptor development server with 🔥 hot reload 🔥")
	prepareBinDirectory()
	rebuild()

	setWatcher()
	defer unsetWatcher()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					unsetWatcher()
					rebuild()
					setWatcher()
				}
			case err := <-watcher.Errors:
				fmt.Println("Error:", err)
			}
		}
	}()

	select {}
}

func setWatcher() {
	ignoreDirectories = readRaptorIgnore()

	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("%sError creating watcher: %v%s\n", colorRed, err, colorNone)
		os.Exit(1)
	}

	if err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println("Error:", err)
			return err
		}

		if info.IsDir() {
			if shouldIgnorePath(path, ignoreDirectories) {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	}); err != nil {
		fmt.Printf("%sError walking directory: %v%s\n", colorRed, err, colorNone)
		os.Exit(1)
	}
}

func unsetWatcher() {
	if watcher != nil {
		watcher.Close()
	}
}

func readRaptorIgnore() []string {
	ignoreDirectories := make([]string, len(defaultIgnoreDirectories), len(defaultIgnoreDirectories)*2)
	copy(ignoreDirectories, defaultIgnoreDirectories)

	content, err := os.ReadFile(".raptorignore")
	if err != nil {
		return ignoreDirectories
	}

	existing := make(map[string]bool, len(defaultIgnoreDirectories))
	for _, dir := range defaultIgnoreDirectories {
		existing[dir] = true
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !existing[line] {
			ignoreDirectories = append(ignoreDirectories, line)
			existing[line] = true
		}
	}

	return ignoreDirectories
}

func shouldIgnorePath(path string, ignoreDirectories []string) bool {
	for _, pattern := range ignoreDirectories {
		if strings.Contains(pattern, string(os.PathSeparator)) {
			if path == pattern {
				return true
			}
			continue
		}

		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func prepareBinDirectory() {
	if _, err := os.Stat("bin"); os.IsNotExist(err) {
		os.Mkdir("bin", 0755)
	}
}

func rebuild() error {
	stop()
	fmt.Println("Rebuilding application... 🏗️")
	cmd := exec.Command("go", "build", "-o", "bin/raptorapp")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	buildStart := time.Now()
	err := cmd.Run()
	buildDuration := time.Since(buildStart)

	if err != nil {
		fmt.Printf("%sBuild failed%s ❌\n", colorRed, colorNone)
		return err
	}

	if buildDuration >= time.Second {
		fmt.Printf("%sBuild completed in %.3fs%s ✅\n", colorGreen, buildDuration.Seconds(), colorNone)
	} else {
		fmt.Printf("%sBuild completed in %dms%s ✅\n", colorGreen, buildDuration.Milliseconds(), colorNone)
	}
	start()

	return nil
}

func start() {
	env := []string{"RAPTOR_DEVELOPMENT=true"}
	runningCmd = exec.Command("bin/raptorapp")
	runningCmd.Env = append(os.Environ(), env...)
	runningCmd.Dir = "."
	runningCmd.Stdout = os.Stdout
	runningCmd.Stderr = os.Stderr
	err := runningCmd.Start()

	if err != nil {
		fmt.Printf("Error starting: %s\n", err)
		os.Exit(1)
	}
}

func stop() {
	if runningCmd != nil && runningCmd.Process != nil {
		if runningCmd.ProcessState != nil && runningCmd.ProcessState.Exited() {
			return
		}
		runningCmd.Process.Signal(os.Interrupt)
		if err := runningCmd.Wait(); err != nil {
			fmt.Println("Error waiting for process:", err)
		}
	}
}
