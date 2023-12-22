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

func developmentServer(cmd *cobra.Command, args []string) {
	_, err := os.Stat(".raptor.toml")
	if err != nil {
		fmt.Println("Please run this command in the root of Raptor project")
		os.Exit(1)
	}

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
					if build() == nil {
						start()
					}
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
	fmt.Println("Building...")
	cmd := exec.Command("go", "build", "-o", "raptorapp")
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func start() {
	fmt.Println("Starting...")
	if runningCmd != nil && runningCmd.Process != nil {
		fmt.Println("Killing process...")
		err := runningCmd.Process.Signal(os.Interrupt)
		if err != nil {
			fmt.Println("Error killing process:", err)
			os.Exit(1)
		}
		fmt.Println("Waiting for process to exit...")
		err = runningCmd.Wait()
	}

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

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long:  `Start the development server with hot reload.`,
	Run:   developmentServer,
}

func init() {
	rootCmd.AddCommand(devCmd)
}
