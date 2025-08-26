package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"zecx-deploy/internal/covert"
)

func main() {
	// start a simple local collector for demo
	h := http.NewServeMux()
	h.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		log.Println("collector: got POST")
		w.WriteHeader(200)
	})
	// reuse metrics endpoint path
	h.HandleFunc("/wss", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	srv := &http.Server{Addr: ":8088", Handler: h}
	go func() {
		log.Println("demo collector listening on :8088")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("collector failed: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := covert.Options{
		Endpoint:          "http://127.0.0.1:8088/hook",
		Token:             "demo-pair",
		HeartbeatInterval: 5 * time.Second,
		MetricsAddr:       ":9090",
	}
	go func() {
		if err := covert.StartTunnel(ctx, opts); err != nil {
			log.Printf("StartTunnel exited: %v", err)
		}
	}()

	// wait for startup
	time.Sleep(200 * time.Millisecond)

	fmt.Println("Sending demo payload")
	if err := covert.SendData(opts.Endpoint, opts.Token, []byte("demo-payload")); err != nil {
		log.Printf("SendData error: %v", err)
	}

	fmt.Println("Demo running. Metrics at http://localhost:9090/metrics")
	time.Sleep(2 * time.Second)

	// shutdown
	ctx.Done()
	_ = srv.Close()
}
