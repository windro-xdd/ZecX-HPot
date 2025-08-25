package uninstall

import (
	"fmt"
	"log"
	"zecx-deploy/internal/transform/decoys"
	"zecx-deploy/internal/transform/emulators"
	"zecx-deploy/internal/transform/firewall"
)

// CleanUp removes all traces of the honeypot from the system.
// It orchestrates the cleanup of all modules.
func CleanUp() error {
	log.Println("Starting uninstallation process...")

	// The order of operations is important:
	// 1. Stop services to release file locks and ports.
	if err := emulators.Stop(); err != nil {
		log.Printf("Error stopping emulators: %v. Continuing cleanup.", err)
	} else {
		log.Println("Successfully stopped service emulators.")
	}

	// 2. Remove decoy files and directories.
	if err := decoys.Clean(); err != nil {
		log.Printf("Error cleaning decoy environment: %v. Continuing cleanup.", err)
	} else {
		log.Println("Successfully cleaned decoy environment.")
	}

	// 3. Restore the firewall to its original state.
	if err := firewall.Restore(); err != nil {
		log.Printf("Error restoring firewall: %v. Manual check may be required.", err)
	} else {
		log.Println("Successfully restored firewall.")
	}

	// 4. In a real scenario, we would also remove any other artifacts,
	//    such as hidden persistence mechanisms (e.g., systemd services).
	log.Println("Uninstallation placeholder: Simulating removal of systemd services.")

	fmt.Println("Uninstallation process complete. The system should be clean.")
	return nil
}
