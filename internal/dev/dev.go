package dev

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

var (
	watcher    *fsnotify.Watcher
	runningCmd *exec.Cmd
)

const (
	colorGreen = "\033[0;32m"
	colorRed   = "\033[0;31m"
	colorNone  = "\033[0m"
)

var Cmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long:  `Start the development server with hot reload.`,
	Run:   developmentServer,
}

func developmentServer(cmd *cobra.Command, args []string) {
	configFiles := []string{
		".raptor.toml",
		".raptor.conf",
		".raptor.prod.toml",
		".raptor.prod.conf",
		".raptor.dev.toml",
		".raptor.dev.conf",
	}

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
	watcher, _ = fsnotify.NewWatcher()
	filepath.Walk(".", watchDir)
	err := watcher.Add(".")
	if err != nil {
		fmt.Println("Error setting watcher:", err)
		os.Exit(1)
	}
}

func unsetWatcher() {
	if watcher != nil {
		watcher.Close()
	}
}

func watchDir(path string, info os.FileInfo, err error) error {
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	if info.IsDir() {
		return watcher.Add(path)
	}
	return nil
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
