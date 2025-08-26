package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"strconv"
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
	collectorEndpoint := flag.String("collector-endpoint", "https://example-collector.local/hook", "Collector endpoint URL for outbound data (https)")
	collectorCA := flag.String("collector-ca", "", "Path to PEM file containing CA(s) to trust for collector TLS (optional)")
	queueDir := flag.String("queue-dir", "covert_queue", "Path to on-disk queue directory for outbound messages")
	maxQueueBytes := flag.Int64("max-queue-bytes", 0, "Maximum size in bytes for the on-disk queue (0 = unlimited)")
	heartbeatSeconds := flag.Int("heartbeat-interval", 30, "Heartbeat interval in seconds for collector keepalive")
	metricsAddr := flag.String("metrics-addr", "", "Optional address to serve Prometheus metrics (e.g. :9090)")
	allowSelfDestruct := flag.Bool("allow-selfdestruct", false, "Allow the background process to remove the original binary when running")
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

	// Propagate selected flags into the background child via environment variables
	if *collectorEndpoint != "" {
		os.Setenv("ZECX_COLLECTOR_ENDPOINT", *collectorEndpoint)
	}
	if *collectorCA != "" {
		os.Setenv("ZECX_COLLECTOR_CA", *collectorCA)
	}
	if *queueDir != "" {
		os.Setenv("ZECX_QUEUE_DIR", *queueDir)
	}
	if *maxQueueBytes > 0 {
		os.Setenv("ZECX_MAX_QUEUE_BYTES", fmt.Sprintf("%d", *maxQueueBytes))
	}
	if *heartbeatSeconds > 0 {
		os.Setenv("ZECX_HEARTBEAT_INTERVAL", fmt.Sprintf("%d", *heartbeatSeconds))
	}
	if *metricsAddr != "" {
		os.Setenv("ZECX_METRICS_ADDR", *metricsAddr)
	}
	if *allowSelfDestruct {
		os.Setenv("ZECX_ALLOW_SELFDESTRUCT", "1")
	}
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
	// Use the new covert.Options API. The Endpoint should be configured by the operator
	// in a future CLI flag or config; for now we provide a placeholder and include the
	// pairing code in the token field so the stub has access.
	// Read operator-supplied collector endpoint and self-destruct flag from env.
	// Flags from the original foreground process are passed to the background via os.Args
	endpoint := os.Getenv("ZECX_COLLECTOR_ENDPOINT")
	if endpoint == "" {
		// fallback to a default; operator should override via CLI when launching.
		endpoint = "https://example-collector.local/hook"
	}

	// Respect the self-destruct permission if the operator allowed it.
	if os.Getenv("ZECX_ALLOW_SELFDESTRUCT") == "1" {
		// attempt to self-destruct now (will no-op if gates not met)
		if err := stealth.SelfDestruct(); err != nil {
			log.Printf("SelfDestruct error: %v", err)
		}
	}

	// Read queue and heartbeat settings from the environment (set by the parent)
	qdir := os.Getenv("ZECX_QUEUE_DIR")
	maxQ := int64(0)
	if v := os.Getenv("ZECX_MAX_QUEUE_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			maxQ = n
		}
	}
	hb := 30 * time.Second
	if v := os.Getenv("ZECX_HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hb = time.Duration(n) * time.Second
		}
	}

	opts := covert.Options{
		Endpoint:          endpoint,
		Token:             pairingCode,
		HeartbeatInterval: hb,
		CABundlePath:      os.Getenv("ZECX_COLLECTOR_CA"),
		QueueDir:          qdir,
		MaxQueueBytes:     maxQ,
	MetricsAddr:       os.Getenv("ZECX_METRICS_ADDR"),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := covert.StartTunnel(ctx, opts); err != nil {
		log.Printf("Covert tunnel failed: %v", err)
	}
}
