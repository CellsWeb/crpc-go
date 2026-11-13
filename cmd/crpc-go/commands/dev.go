package commands

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// DevServer runs the project in development mode with continuous file watching and <200ms restart
func DevServer(projectDir string) error {
	fmt.Println("🚀 Starting CellsWeb RPC Development Server...")

	var cmd *exec.Cmd
	stopProcess := func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}

	startProcess := func() {
		stopProcess()
		fmt.Println("\n⚡ Rebuilding and starting server...")
		cmd = exec.Command("go", "run", "cmd/server/main.go")
		cmd.Dir = projectDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed starting server: %v\n", err)
		}
	}

	startProcess()

	// Graceful shutdown handler
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		stopProcess()
		os.Exit(0)
	}()

	// Continuous File Watcher
	lastModMap := make(map[string]time.Time)
	scanFiles := func() (bool, error) {
		changed := false
		err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Skip hidden dirs, dist, node_modules, .git
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "dist" || base == "node_modules" || base == "vendor" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() && (strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".env")) {
				modTime := info.ModTime()
				if prevTime, exists := lastModMap[path]; exists {
					if modTime.After(prevTime) {
						changed = true
						lastModMap[path] = modTime
					}
				} else {
					lastModMap[path] = modTime
				}
			}
			return nil
		})
		return changed, err
	}

	// Initialize file timestamps
	_, _ = scanFiles()

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		changed, _ := scanFiles()
		if changed {
			startProcess()
		}
	}

	return nil
}
