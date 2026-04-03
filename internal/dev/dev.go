package dev

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

const (
	colorGreen = "\033[0;32m"
	colorRed   = "\033[0;31m"
	colorNone  = "\033[0m"

	debounceDuration = 300 * time.Millisecond
)

var binaryPath string

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

func init() {
	binaryName := "raptorapp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath = filepath.Join("bin", binaryName)
}

func isRaptorProject() bool {
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			return true
		}
	}
	return false
}

func developmentServer(cmd *cobra.Command, args []string) {
	if !isRaptorProject() {
		fmt.Println("Please run this command in the root of Raptor project")
		os.Exit(1)
	}

	fmt.Println("Starting 🦖 Raptor development server with live reload ⚡")
	prepareBinDirectory()

	var runningCmd *exec.Cmd
	rebuild := func() {
		runningCmd = stopAndRebuild(runningCmd)
	}

	rebuild()

	ignoreDirs := readRaptorIgnore()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("%sError creating watcher: %v%s\n", colorRed, err, colorNone)
		os.Exit(1)
	}
	defer watcher.Close()

	watchDirectories(watcher, ignoreDirs)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Add newly created directories to the watcher
				if event.Op&fsnotify.Create != 0 {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						if !shouldIgnorePath(event.Name, ignoreDirs) {
							watcher.Add(event.Name)
						}
					}
				}
				debounce.Reset(debounceDuration)
			}
		case <-debounce.C:
			rebuild()
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Println("Error:", err)
		case <-sigCh:
			stop(runningCmd)
			return
		}
	}
}

func watchDirectories(watcher *fsnotify.Watcher, ignoreDirs []string) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldIgnorePath(path, ignoreDirs) {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%sError walking directory: %v%s\n", colorRed, err, colorNone)
		os.Exit(1)
	}
}

func readRaptorIgnore() []string {
	existing := make(map[string]bool, len(defaultIgnoreDirectories))
	dirs := make([]string, len(defaultIgnoreDirectories), len(defaultIgnoreDirectories)*2)
	copy(dirs, defaultIgnoreDirectories)
	for _, dir := range defaultIgnoreDirectories {
		existing[dir] = true
	}

	content, err := os.ReadFile(".raptorignore")
	if err != nil {
		return dirs
	}

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !existing[line] {
			dirs = append(dirs, line)
			existing[line] = true
		}
	}

	return dirs
}

func shouldIgnorePath(path string, ignoreDirs []string) bool {
	normalizedPath := filepath.ToSlash(path)
	base := filepath.Base(normalizedPath)
	for _, pattern := range ignoreDirs {
		normalizedPattern := filepath.ToSlash(pattern)
		if normalizedPath == normalizedPattern || base == normalizedPattern {
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

func stopAndRebuild(runningCmd *exec.Cmd) *exec.Cmd {
	stop(runningCmd)
	fmt.Println("Rebuilding application... 🏗️")
	cmd := exec.Command("go", "build", "-o", binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	buildStart := time.Now()
	err := cmd.Run()
	buildDuration := time.Since(buildStart)

	if err != nil {
		fmt.Printf("%sBuild failed%s ❌\n", colorRed, colorNone)
		return nil
	}

	if buildDuration >= time.Second {
		fmt.Printf("%sBuild completed in %.3fs%s ✅\n", colorGreen, buildDuration.Seconds(), colorNone)
	} else {
		fmt.Printf("%sBuild completed in %dms%s ✅\n", colorGreen, buildDuration.Milliseconds(), colorNone)
	}

	return start()
}

func start() *exec.Cmd {
	cmd := exec.Command(binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting: %s\n", err)
		os.Exit(1)
	}
	return cmd
}

func stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		cmd.Process.Kill()
	}
	if err := cmd.Wait(); err != nil {
		if !strings.Contains(err.Error(), "interrupt") {
			fmt.Println("Error waiting for process to stop:", err)
		}
	}
}
