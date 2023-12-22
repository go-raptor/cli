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

	watcher, _ = fsnotify.NewWatcher()
	defer watcher.Close()

	filepath.Walk(".", watchDir)

	//recompileAndRestart()

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					recompileAndRestart()
				}
			case err := <-watcher.Errors:
				fmt.Println("Error:", err)
			}
		}
	}()

	err = watcher.Add(".")
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	select {}
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

func recompileAndRestart() {
	fmt.Println("File changed. Recompiling and restarting...")
	watcher.Close()

	if runningCmd != nil && runningCmd.Process != nil {
		fmt.Println("Killing process...")
		err := runningCmd.Process.Signal(os.Interrupt)
		if err != nil {
			fmt.Println("Error killing process:", err)
			fmt.Println("Restart aborted.")
			os.Exit(1)
		}
		fmt.Println("Waiting for process to exit...")
		err = runningCmd.Wait()
	}

	runningCmd = exec.Command("go", "run", "main.go")
	runningCmd.Dir = "."
	runningCmd.Stdout = os.Stdout
	runningCmd.Stderr = os.Stderr
	err := runningCmd.Start()

	if err != nil {
		fmt.Printf("Error compiling: %s\n", err)
		fmt.Println("Restart aborted.")
		return
	}

	fmt.Println("Successfully recompiled and restarted.")

	watcher, _ = fsnotify.NewWatcher()
	filepath.Walk(".", watchDir)
	watcher.Add(".")
}

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long:  `Start the development server with hot reload.`,
	Run:   developmentServer,
}

func init() {
	rootCmd.AddCommand(devCmd)
}
