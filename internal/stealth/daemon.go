package stealth

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

const (
	backgroundEnvVar  = "ZECX_BACKGROUND_TASK"
	pairingCodeEnvVar = "ZECX_PAIRING_CODE"
)

// IsBackground returns true if the process is the background child.
func IsBackground() bool {
	return os.Getenv(backgroundEnvVar) == "1"
}

// GetPairingCode retrieves the pairing code from the environment.
func GetPairingCode() string {
	return os.Getenv(pairingCodeEnvVar)
}

// Daemonize forks the process into the background, passing the pairing code.
// It returns true if the current process is the child that should continue execution,
// and false if it's the parent that should exit.
func Daemonize(pairingCode string) bool {
	// This function should only be called by the parent process.
	// The check for whether we ARE the background process is now in IsBackground().
	args := os.Args[1:]
	cmd := exec.Command(os.Args[0], args...)

	// Set the environment variables to mark the child process and pass the code.
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", backgroundEnvVar))
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", pairingCodeEnvVar, pairingCode))

	// Detach the process from the current terminal.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the new process.
	err := cmd.Start()
	if err != nil {
		// Use log instead of fmt.Fprintf for consistency
		log.Printf("Failed to launch background process: %v\n", err)
		// If we can't fork, we must exit to prevent running in the foreground.
		os.Exit(1)
	}

	// Parent process exits successfully.
	log.Printf("Background process launched with PID: %d", cmd.Process.Pid)
	fmt.Printf("Background process launched with PID: %d\n", cmd.Process.Pid)
	return false
}

// SelfDestruct removes the original executable file.
func SelfDestruct() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	log.Printf("Attempting to self-destruct: %s", exePath)
	return os.Remove(exePath)
}
