package stealth

import (
	"fmt"
	"os"
	"os/exec"
)

// Daemonize forks the process into the background.
// It returns true if the current process is the child that should continue execution,
// and false if it's the parent that should exit.
func Daemonize() bool {
	// If this environment variable is set, we are the child process.
	if os.Getenv("ZECX_BACKGROUND_TASK") == "1" {
		return true
	}

	// Prepare the command to re-execute the program.
	args := os.Args[1:]
	cmd := exec.Command(os.Args[0], args...)

	// Set the environment variable to mark the child process.
	cmd.Env = append(os.Environ(), "ZECX_BACKGROUND_TASK=1")

	// Detach the process from the current terminal.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the new process.
	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch background process: %v\n", err)
		// If we can't fork, we must exit to prevent running in the foreground.
		os.Exit(1)
	}

	// Parent process exits successfully.
	fmt.Printf("Background process launched with PID: %d\n", cmd.Process.Pid)
	return false
}

// SelfDestruct removes the original executable file.
func SelfDestruct() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	return os.Remove(exePath)
}
