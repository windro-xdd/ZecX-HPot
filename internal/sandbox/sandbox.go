package sandbox

import "log"

// Package sandbox contains scaffolding for isolating emulated services.
// NOTE: Implementing real sandboxing requires privileged operations and
// careful security review. These helpers are safe no-ops by default and
// exist to centralize future work.

// Config holds options for sandbox creation.
type Config struct {
	// TODO: add namespace, cgroup, seccomp policy fields
	Name string
}

// Setup prepares an isolated environment for a service. It is intentionally
// conservative: by default, it logs the requested setup and returns nil.
func Setup(cfg Config) error {
	log.Printf("[sandbox] setup requested: %v (no-op placeholder)", cfg)
	// Future: create new mount, pid, net namespaces; configure cgroups; apply seccomp.
	return nil
}

// Teardown cleans up any allocated sandbox resources.
func Teardown(cfg Config) error {
	log.Printf("[sandbox] teardown requested: %v (no-op placeholder)", cfg)
	return nil
}
