# PHASES and Steps

This file lists phases and concrete steps to complete the honeypot project. Use it as a checklist while developing.

Phase 1 — Scaffold (completed by this commit)
- Create Cargo project and dependencies
- Add modules: config, fsgen, listener, reporter, util
- Add README, PHASES.md, config example

Phase 2 — Core features
- Harden and test TCP listeners for large port ranges
- Improve deceptive banners per-service
- Extend protocol emulation (basic SSH, FTP, HTTP interactions)
- Add UDP listeners for certain ports optionally

Phase 3 — Filesystem realism
- Add templates for /etc, /var/log entries
- Seed SSH authorized_keys, sudoers-like files
- Randomized timestamps and ownership metadata (requires running as root or using user namespaces)

Phase 4 — Logging and reporting
- Encrypt logs at rest
- Authenticate to backend and rotate pairing code
- Add backpressure handling when logs are large

Phase 5 — Security and containment
- Implement and safely apply iptables/nft rules
- Add resource limits for connections (rate limits)
- Run under unprivileged user and sandbox (namespaces)

Phase 6 — Tests and CI
- Unit tests for fs generator, listeners (integration), reporter (mock server)
- Add GitHub Actions for lint/build/test

Phase 7 — Packaging
- Create Debian package or container image (Dockerfile)

Notes
- Always test network rules on isolated VMs to avoid locking yourself out.
- Do not deploy honeypots with real credentials.
