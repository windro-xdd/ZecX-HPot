# ZecX-Honeypot Automation Engine - Development Plan

This document outlines the phased development plan for the `zecx-deploy` tool, a comprehensive honeypot automation engine.

## Project Vision
`zecx-deploy` is a command-line tool that fully automates the transformation of a standard Linux VM into a high-interaction, secure, and stealthy honeypot system. The goal is to make deployment incredibly simple for non-experts while providing a realistic environment to attackers, enabling the secure collection of high-quality threat intelligence.

---

## Phase 1: Core Framework & User Workflow (Complete)

This phase establishes the foundational structure of the `zecx-deploy` command-line tool and the primary user interaction flow.

*   **[✓] Project Structure:** Create a standard Go project layout (`cmd`, `internal`).
*   **[✓] CLI Argument Parsing:** Implement command-line flag handling, starting with the `--uninstall` flag.
*   **[✓] T&C Acceptance Module (`internal/cli`):**
    *   Display the terms and conditions prompt.
    *   Block execution until the user explicitly types "yes".
    *   Terminate gracefully if the user declines.
*   **[✓] Pairing Code Generator (`internal/pairing`):**
    *   Generate a cryptographically secure, human-readable pairing code (e.g., `adjective-noun-number`).
    *   Display the code clearly to the user for use in the web dashboard.
*   **[✓] Background Daemonization (`internal/stealth`):**
    *   Implement logic to fork the main process into the background.
    *   Detach the process from the user's terminal, allowing the session to be closed.
    *   Ensure the parent process exits cleanly after forking.
*   **[✓] Main Application Flow (`cmd/zecx-deploy`):**
    *   Integrate all modules from this phase into a coherent startup sequence.

---

## Phase 2: System Transformation & Core Modules

This phase focuses on building out the core components responsible for transforming the host system into a honeypot. Initial implementations will be placeholders that log their actions, with full functionality to be added in later phases.

*   **[✓] Firewall Controller (`internal/transform/firewall`):**
    *   **Goal:** Programmatically manage `iptables` or `nftables`.
    *   **Task:** Create a `Configure()` function that will eventually hold the logic for redirecting traffic from standard ports (22, 80, 443, etc.) to the high-port listeners of the service emulators.
*   **[✓] High-Interaction Service Emulators (`internal/transform/emulators`):**
    *   **Goal:** Launch concurrent daemons that mimic real services.
    *   **Task:** Create a `Start()` function that will launch goroutines for each service emulator (SSH, HTTP, FTP, SMB). Initially, these will be simple listeners that log connection attempts.
*   **[✓] Decoy Environment Seeder (`internal/transform/decoys`):**
    *   **Goal:** Create a believable filesystem to deceive attackers.
    *   **Task:** Create a `Seed()` function to manage the creation of fake user directories, bash histories, log files, and application configs.
*   **[✓] Secure Uninstall Module (`internal/uninstall`):**
    *   **Goal:** Completely and safely remove all traces of the honeypot.
    *   **Task:** Create a `CleanUp()` function that will orchestrate the reversal of all transformations: stopping services, removing decoy files, and resetting firewall rules.
*   **[✓] Transformation Orchestrator (`internal/transform`):**
    *   **Goal:** Tie all transformation steps together.
    *   **Task:** Create an `Apply()` function that calls the firewall, decoy, and emulator modules in the correct order.

---

## Phase 3: Stealth & Security Hardening

This phase implements the critical non-functional requirements for stealth and security.

*   **[✓] Process Masking (`internal/stealth`):**
    **Goal:** Hide the honeypot processes from the attacker.
    **Task:** Implement logic to change the process name as seen in tools like `ps` and `top` to common system process names (`kworker`, `dbus-daemon`, etc.).
    **Status:** implemented best-effort `MaskProcess()` that writes `/proc/self/comm` and falls back to prctl PR_SET_NAME on Linux.

*   **[✓] Binary Self-Destruction (`internal/stealth`):**
    **Goal:** Remove the initial executable to hide the entry vector.
    **Task:** `SelfDestruct()` now exists with safety gates: it only removes the binary when running as the background child and when `ZECX_ALLOW_SELFDESTRUCT=1` is set.

*   **[~] Service Sandboxing (`internal/transform/emulators`):**
    **Goal:** Isolate emulated services from the host system.
    **Task:** Scaffolding added in `internal/sandbox` with interfaces and TODOs for namespaces, cgroups, and seccomp. Implementation requires careful security review.


---

## Phase 4: Covert Communications

This phase focuses on building the secure, outbound-only communication channel for data exfiltration.

*   **[ ] Encrypted Reverse Tunnel (`internal/covert`):**
    *   **Goal:** Establish a persistent, undetectable link to the monitoring dashboard.
    *   **Task:** Implement a `StartTunnel()` function that initiates an outbound WebSocket over TLS (WSS) connection to a remote server on port 443. This avoids opening any inbound ports.
*   **[ ] Data Exfiltration Protocol:**
    *   **Goal:** Securely transmit captured data.
    *   **Task:** Design and implement a simple protocol to send the pairing code for authentication, followed by a stream of log data (connection attempts, commands entered, files downloaded) from the emulators.

---

## Phase 5: Full-Fidelity Service Emulators

This phase involves replacing the placeholder service emulators with robust, high-interaction versions.

*   **[ ] SSH Emulator:**
    *   **Goal:** Emulate a full SSH server.
    *   **Task:** Implement an emulator that can handle key exchange, authentication (logging credentials), and shell session interaction, capturing all commands executed by the attacker.
*   **[ ] HTTP/S Emulator:**
    *   **Goal:** Serve decoy web pages.
    *   **Task:** Implement an emulator that can serve fake web pages, log request headers and bodies, and potentially mimic common vulnerabilities.
*   **[ ] FTP & SMB Emulators:**
    *   **Goal:** Emulate file-sharing services.
    *   **Task:** Implement emulators that allow attackers to connect, list files, and upload/download decoy files, logging all interactions.

---

## Phase 6: Integration, Testing, and Finalization

This is the final phase to bring all components together into a production-ready tool.

*   **[ ] Integration:** Connect the real service emulators to the covert tunneling module to ensure all captured data is exfiltrated correctly.
*   **[ ] End-to-End Testing:** Perform a full deployment on a clean VM to validate the entire user workflow, from running the binary to seeing data appear on a test dashboard.
*   **[ ] Security Audit:** Review all code for potential vulnerabilities, paying special attention to the sandboxing implementation and the covert channel.
## Addendum: SSH Pseudo-Shell and Interaction Improvements

This section outlines future work related to the SSH emulator and its interaction capabilities.

*   **[ ] SSH Pseudo-Shell:** 
    *   **Goal:** Enhance the SSH emulator to provide a more realistic shell experience.
    *   **Task:** Implement features that allow for command history, session logging, and interaction tracking.

*   **[ ] Finalize `README.md`:** Update this document to be a comprehensive user manual for the final product.
