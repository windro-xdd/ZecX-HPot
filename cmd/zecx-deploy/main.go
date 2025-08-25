package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"zecx-deploy/internal/cli"
	"zecx-deploy/internal/covert"
	"zecx-deploy/internal/pairing"
	"zecx-deploy/internal/stealth"
	"zecx-deploy/internal/transform"
	"zecx-deploy/internal/uninstall"
)

func main() {
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall the ZecX-Honeypot and restore the system.")
	flag.Parse()

	// Set up logging. In a real scenario, this would write to a hidden, rotated file.
	logFile, err := os.OpenFile("zecx-honeypot.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Println("ZecX-Honeypot starting...")

	if *uninstallFlag {
		if err := uninstall.CleanUp(); err != nil {
			log.Printf("Error during uninstallation: %v\n", err)
			fmt.Fprintf(os.Stderr, "Error during uninstallation: %v\n", err)
			os.Exit(1)
		}
		log.Println("ZecX-Honeypot has been successfully uninstalled.")
		fmt.Println("ZecX-Honeypot has been successfully uninstalled.")
		return
	}

	if !cli.AcceptTerms() {
		log.Println("Deployment canceled by user at T&C prompt.")
		fmt.Println("Deployment canceled.")
		os.Exit(1)
	}

	code, err := pairing.GenerateCode()
	if err != nil {
		log.Fatalf("Fatal error generating pairing code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Your one-time pairing code is: %s\n", code)
	fmt.Println("Use this on the web dashboard to monitor this honeypot.")
	fmt.Println("The tool will now fork into the background to complete the system transformation.")
	log.Printf("Generated pairing code: %s", code)

	// Fork to background
	if !stealth.Daemonize() {
		// This is the parent process, it can exit now.
		log.Println("Parent process exiting after forking.")
		return
	}

	// From here on, we are in the background process.
	log.Println("--- Background process started ---")

	// 1. Perform System Transformation
	if err := transform.Apply(); err != nil {
		log.Printf("Fatal error during system transformation: %v", err)
		os.Exit(1) // Exit silently
	}

	// 2. Establish Covert Communication (This is a blocking call)
	log.Println("Handing off to covert communication module.")
	covert.StartTunnel(code) // This will run forever
}
