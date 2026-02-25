// Package mobile provides a process-management API for running the Cheapskate
// server on mobile platforms (Android/iOS).
//
// Architecture: The full server binary is cross-compiled for the target platform
// (e.g. GOOS=linux GOARCH=arm64 for Android). The mobile app extracts the binary
// to its private storage and manages it as a subprocess. A WebView points to
// http://localhost:{port} to display the UI.
//
// This approach gives 100% code reuse - the exact same server code runs on
// desktop, Docker, and mobile. No handler duplication or framework translation.
//
// For gomobile bind (JNI integration), this package exposes simple Start/Stop
// functions. The Start function spawns the server binary as a child process.
package mobile

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	process *exec.Cmd
)

// Start launches the server binary as a subprocess.
// binaryPath is the full path to the extracted server binary.
// dataDir is the app's private storage directory for the database.
// port is the localhost port to listen on (e.g. 8080).
// Returns an error string (empty on success) — gomobile requires string returns, not error.
func Start(binaryPath string, dataDir string, port int) string {
	mu.Lock()
	defer mu.Unlock()

	if process != nil {
		return "server already running"
	}

	// Ensure binary is executable
	if err := os.Chmod(binaryPath, 0755); err != nil {
		return fmt.Sprintf("chmod failed: %v", err)
	}

	// Create data and backup directories
	os.MkdirAll(dataDir, 0755)
	backupDir := dataDir + "/backups"
	os.MkdirAll(backupDir, 0755)

	dbPath := dataDir + "/cheapskate.db"
	categoriesPath := dataDir + "/categories.json"

	// Build command
	cmd := exec.Command(binaryPath,
		"--port", fmt.Sprintf("%d", port),
		"--db", dbPath,
		"--categories", categoriesPath,
		"--backup-path", backupDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Sprintf("failed to start server: %v", err)
	}

	process = cmd

	// Wait for server to be ready (max 10 seconds)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ready := waitForServer(addr, 10*time.Second)
	if !ready {
		log.Printf("Warning: server may not be ready yet on %s", addr)
	}

	log.Printf("Mobile server started (PID %d) on port %d", cmd.Process.Pid, port)
	return ""
}

// Stop gracefully shuts down the server process.
// Returns an error string (empty on success).
func Stop() string {
	mu.Lock()
	defer mu.Unlock()

	if process == nil {
		return ""
	}

	// Send interrupt signal for graceful shutdown
	if err := process.Process.Signal(os.Interrupt); err != nil {
		// If interrupt fails, force kill
		process.Process.Kill()
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- process.Wait()
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Force kill after timeout
		process.Process.Kill()
		<-done
	}

	process = nil
	log.Println("Mobile server stopped")
	return ""
}

// IsRunning returns true if the server process is currently running.
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return process != nil
}

// waitForServer polls the server until it responds or timeout.
func waitForServer(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			// Double-check with HTTP
			resp, err := http.Get("http://" + addr + "/")
			if err == nil {
				resp.Body.Close()
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
