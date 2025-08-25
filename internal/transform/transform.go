package transform

import (
	"fmt"
	"log"
	"zecx-deploy/internal/transform/decoys"
	"zecx-deploy/internal/transform/emulators"
	"zecx-deploy/internal/transform/firewall"
)

// Apply runs the full system transformation.
func Apply() error {
	log.Println("Starting system transformation...")

	// 1. Configure firewall
	if err := firewall.Configure(); err != nil {
		return fmt.Errorf("failed to configure firewall: %w", err)
	}

	// 2. Seed decoy environment
	if err := decoys.Seed(); err != nil {
		return fmt.Errorf("failed to seed decoy environment: %w", err)
	}

	// 3. Launch service emulators
	// This will start the emulators in the background.
	if err := emulators.Start(); err != nil {
		return fmt.Errorf("failed to start service emulators: %w", err)
	}

	log.Println("System transformation complete. Honeypot is now active.")
	return nil
}
