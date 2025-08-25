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
	logFile, err := os.OpenFile("zecx-honeypot.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Println("ZecX-Honeypot instance starting...")

	// ...existing startup code...

	if stealth.IsBackground() {
		runBackgroundTasks()
		return
	}

	runForegroundTasks()
}

func runForegroundTasks() {
	// Support both a flag and a subcommand for uninstalling.
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall the ZecX-Honeypot and restore the system.")
	flag.Parse()

	// ...existing flag handling (use --uninstall to run cleanup)

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
		os.Exit(0)
	}

	code, err := pairing.GenerateCode()
	if err != nil {
		log.Fatalf("Fatal error generating pairing code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Your one-time pairing code is: %s\n", code)
	fmt.Println("Use this on the web dashboard to monitor this honeypot.")
	fmt.Println("The tool will now fork into the background to complete the system transformation.")
	log.Printf("Generated pairing code: %s. Forking to background.", code)

	stealth.Daemonize(code)
}

func runBackgroundTasks() {
	log.Println("--- Background process started ---")

	pairingCode := stealth.GetPairingCode()
	if pairingCode == "" {
		log.Println("FATAL: Background process started without a pairing code.")
		os.Exit(1)
	}
	log.Printf("Background process operating with pairing code: %s", pairingCode)

	if err := transform.Apply(); err != nil {
		log.Printf("FATAL: Error during system transformation: %v", err)
		uninstall.CleanUp()
		os.Exit(1)
	}

	log.Println("Handing off to covert communication module.")
	covert.StartTunnel(pairingCode)
}
