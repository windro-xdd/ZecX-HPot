package firewall

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// PortMapping defines a redirection from a source port to a target port.
type PortMapping struct {
	SourcePort int
	TargetPort int
	Protocol   string
}

var mappings = []PortMapping{
	{SourcePort: 21, TargetPort: 2121, Protocol: "tcp"}, // FTP
	{SourcePort: 22, TargetPort: 2222, Protocol: "tcp"}, // SSH
	{SourcePort: 80, TargetPort: 8080, Protocol: "tcp"}, // HTTP
	{SourcePort: 443, TargetPort: 8443, Protocol: "tcp"},// HTTPS
	{SourcePort: 445, TargetPort: 4445, Protocol: "tcp"},// SMB
}

// runIptables executes an iptables command and logs its output.
func runIptables(args ...string) error {
	cmd := exec.Command("iptables", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("iptables command failed: iptables %s", strings.Join(args, " "))
		log.Printf("Output: %s", string(output))
		return fmt.Errorf("iptables error: %w", err)
	}
	log.Printf("iptables command successful: iptables %s", strings.Join(args, " "))
	return nil
}

// Configure sets up the firewall rules to redirect traffic to the honeypot emulators.
func Configure() error {
	log.Println("Initializing firewall configuration...")

	// Check if iptables is available
	if _, err := exec.LookPath("iptables"); err != nil {
		log.Println("iptables command not found, skipping firewall configuration. This may be expected on non-Linux systems.")
		return nil // Not a fatal error, allows testing on Windows/macOS
	}

	for _, m := range mappings {
		// Add rule to the PREROUTING chain in the nat table
		args := []string{
			"-t", "nat",
			"-A", "PREROUTING",
			"-p", m.Protocol,
			"--dport", fmt.Sprintf("%d", m.SourcePort),
			"-j", "REDIRECT",
			"--to-port", fmt.Sprintf("%d", m.TargetPort),
		}
		if err := runIptables(args...); err != nil {
			// If one rule fails, log it but try to apply the others
			log.Printf("Failed to apply firewall rule for port %d: %v", m.SourcePort, err)
		}
	}

	fmt.Println("Firewall configured.")
	return nil
}

// Restore resets the firewall rules to their original state.
func Restore() error {
	log.Println("Restoring original firewall rules...")

	if _, err := exec.LookPath("iptables"); err != nil {
		log.Println("iptables command not found, skipping firewall restoration.")
		return nil
	}

	for _, m := range mappings {
		// Delete the rule from the PREROUTING chain
		args := []string{
			"-t", "nat",
			"-D", "PREROUTING",
			"-p", m.Protocol,
			"--dport", fmt.Sprintf("%d", m.SourcePort),
			"-j", "REDIRECT",
			"--to-port", fmt.Sprintf("%d", m.TargetPort),
		}
		// We don't treat errors here as fatal, as a rule might not exist if setup failed.
		_ = runIptables(args...)
	}

	log.Println("Firewall restoration complete.")
	return nil
}
