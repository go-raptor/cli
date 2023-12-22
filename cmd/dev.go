package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"

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

func developmentServer(cmd *cobra.Command, args []string) {
	_, err := os.Stat(".raptor.toml")
	if err != nil {
		fmt.Println("Please run this command in the root of Raptor project")
		os.Exit(1)
	}

	fmt.Println("Starting Raptor development server with 🔥 hot reload 🔥")
	rebuildAndStart()

	setWatcher()
	defer watcher.Close()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					if watcher != nil {
						watcher.Close()
					}
					rebuildAndStart()
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

func build() error {
	stop()
	fmt.Println("Rebuilding application... 🏗️")
	cmd := exec.Command("go", "build", "-o", "raptorapp")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func start() {
	runningCmd = exec.Command("./raptorapp")
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
			os.Exit(1)
		}
	}
}

func rebuildAndStart() {
	if err := build(); err == nil {
		fmt.Println(colorGreen, "Build successful ✅", colorNone)
		start()
	} else {
		fmt.Println(colorRed, "Build failed ❌", colorNone)
	}
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long:  `Start the development server with hot reload.`,
	Run:   developmentServer,
}

func init() {
	rootCmd.AddCommand(devCmd)
}
