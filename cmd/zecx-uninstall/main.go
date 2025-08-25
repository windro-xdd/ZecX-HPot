package main

import (
    "fmt"
    "log"
    "os"
    "zecx-deploy/internal/uninstall"
)

func main() {
    logFile, err := os.OpenFile("zecx-uninstall.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        fmt.Fprintf(os.Stderr, "FATAL: Failed to open log file: %v\n", err)
        os.Exit(1)
    }
    defer logFile.Close()
    log.SetOutput(logFile)
    log.Println("ZecX-Uninstall helper starting...")

    if err := uninstall.CleanUp(); err != nil {
        log.Printf("Uninstall failed: %v", err)
        fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
        os.Exit(1)
    }

    log.Println("Uninstall completed successfully")
    fmt.Println("Uninstall completed successfully")
}
