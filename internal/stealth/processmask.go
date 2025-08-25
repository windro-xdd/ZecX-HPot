package stealth

import (
    "log"
    "os"
)

// MaskProcess attempts to change the visible process name for simple tools (best-effort).
// This is intentionally conservative: it will try to write to /proc/self/comm when available
// and otherwise log the intent. Full process name hiding is platform-specific and may require
// external libraries or capabilities.
func MaskProcess(name string) error {
    log.Printf("[stealth] Masking process as %s (best-effort)", name)
    // Try the Linux /proc/self/comm write (requires appropriate permissions).
    if f, err := os.OpenFile("/proc/self/comm", os.O_WRONLY, 0); err == nil {
        defer f.Close()
        if _, err := f.WriteString(name + "\n"); err != nil {
            log.Printf("[stealth] /proc/self/comm write failed: %v", err)
            return err
        }
        return nil
    }
    // If not available, just return nil (no-op) but keep a log entry so operators know we tried.
    log.Println("[stealth] /proc/self/comm not writable or not present; skipping real mask.")
    return nil
}
