package covert

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"os"
	"path/filepath"
	"sort"
	"crypto/rand"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Package covert implements a conservative outbound-only channel using
// HTTPS POSTs. This provides a TLS-protected, outbound-initiated mechanism
// suitable for environments where opening inbound ports is undesirable.

// Options for the covert channel.
type Options struct {
	Endpoint string // remote endpoint URL (must be https://... or wss://...)
	Token    string // auth token or pairing code
	// Poll interval for keepalive/heartbeat
	HeartbeatInterval time.Duration
	// Optional PEM-encoded CA bundle file path to trust for TLS
	CABundlePath string
	// QueueDir overrides the default on-disk queue directory.
	QueueDir string
	// MaxQueueBytes caps the on-disk queue size (0 == unlimited).
	MaxQueueBytes int64
	// Optional address to serve Prometheus metrics (e.g. ":9090"). Empty = disabled.
	MetricsAddr string
}

// payload is the JSON envelope sent to the remote collector.
type payload struct {
	Pairing string `json:"pairing_code"`
	Type    string `json:"type"`
	Time    int64  `json:"ts"`
	Data    string `json:"data,omitempty"`
}

var (
	// wsMsgCh is a global buffered channel used to queue outbound messages
	// for the active WSS connection. If nil, no WSS writer is present.
	wsMsgCh chan payload
	wsMu    = &sync.RWMutex{}
)

// Prometheus metrics
var (
	metricQueueEnqueued = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_queue_enqueued_total",
		Help: "Total number of messages enqueued to disk queue",
	})
	metricQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zecx_covert_queue_depth",
		Help: "Current number of files in the disk queue",
	})
	metricSendSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_send_success_total",
		Help: "Total number of successful outbound send operations",
	})
	metricSendFailure = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_send_failure_total",
		Help: "Total number of failed outbound send operations",
	})
	metricEnqueueError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_enqueue_errors_total",
		Help: "Total number of errors writing to the disk queue",
	})
	metricWSSSendFailure = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_wss_send_failure_total",
		Help: "Total number of failures sending messages over WSS",
	})
	metricQueueEvictions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zecx_covert_queue_evictions_total",
		Help: "Total number of queue files evicted to enforce max queue size",
	})
)

func init() {
	prometheus.MustRegister(metricQueueEnqueued)
	prometheus.MustRegister(metricQueueDepth)
	prometheus.MustRegister(metricSendSuccess)
	prometheus.MustRegister(metricSendFailure)
	prometheus.MustRegister(metricEnqueueError)
	prometheus.MustRegister(metricWSSSendFailure)
	prometheus.MustRegister(metricQueueEvictions)
}

const queueDir = "covert_queue"

// enqueueToDisk writes the payload as a JSON file to the queue directory.
// enqueueToDisk writes the payload as a JSON file to the provided queue dir.
// If maxBytes > 0, it enforces a cap by removing oldest files until there is
// room for the new message.
func enqueueToDisk(p payload, dir string, maxBytes int64) error {
	if dir == "" {
		dir = queueDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	// Enforce maxBytes if requested
	if maxBytes > 0 {
		var total int64
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, f := range files {
			if fi, err := os.Stat(f); err == nil {
				total += fi.Size()
			}
		}
		// If adding this file would exceed cap, remove oldest files until it's OK
		if total+int64(len(data)) > maxBytes {
			filesSorted := files
			sort.Strings(filesSorted)
			for _, f := range filesSorted {
				if total+int64(len(data)) <= maxBytes {
					break
				}
				if fi, err := os.Stat(f); err == nil {
					total -= fi.Size()
					if err := os.Remove(f); err == nil {
						metricQueueEvictions.Inc()
					}
				}
			}
		}
	}

	// filename: timestamp-nrand.json
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	randb := make([]byte, 6)
	if _, err := rand.Read(randb); err != nil {
		return err
	}
	name := ts + "-" + hex.EncodeToString(randb) + ".json"
	path := filepath.Join(dir, name)
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		metricEnqueueError.Inc()
		return err
	}

	// Metrics + log
	metricQueueEnqueued.Inc()
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	metricQueueDepth.Set(float64(len(files)))
	log.Printf("[covert] enqueued message to disk %s (queue_files=%d)", path, len(files))
	return nil
}

// processDiskQueue periodically scans the queue directory and attempts
// delivery of queued payloads using the same send logic (WSS preferred).
func processDiskQueue(ctx context.Context, opts Options) {
	interval := 5 * time.Second
	if opts.HeartbeatInterval > 0 {
		interval = opts.HeartbeatInterval / 6
		if interval < 1*time.Second {
			interval = 1 * time.Second
		}
	}
	dir := opts.QueueDir
	if dir == "" {
		dir = queueDir
	}
	_ = opts.MaxQueueBytes
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil || len(files) == 0 {
			metricQueueDepth.Set(0)
			continue
		}
		// Update queue depth metric
		metricQueueDepth.Set(float64(len(files)))
		sort.Strings(files)
		for _, f := range files {
			data, err := ioutil.ReadFile(f)
			if err != nil {
				continue
			}
			var p payload
			if err := json.Unmarshal(data, &p); err != nil {
				// malformed file: remove it to avoid infinite loop
				os.Remove(f)
				continue
			}
			// try WSS enqueue first
			wsMu.RLock()
			ch := wsMsgCh
			wsMu.RUnlock()
			sent := false
			if ch != nil {
				select {
				case ch <- p:
					sent = true
				default:
					sent = false
				}
			}
			if !sent {
				client := &http.Client{Timeout: 30 * time.Second}
				if err := postJSON(client, opts.Endpoint, p); err == nil {
					metricSendSuccess.Inc()
					sent = true
				}
			}
			if sent {
				os.Remove(f)
			}
		}
	}
}

// StartTunnel initiates a background loop that periodically POSTs a heartbeat
// to the configured endpoint and sends queued data synchronously. This
// implementation is intentionally simple and conservative: it uses standard
// HTTPS POST with JSON and retries on transient failures with backoff.
// StartTunnel blocks for the life of the honeypot (matching prior behavior).
func StartTunnel(ctx context.Context, opts Options) error {
	if opts.Endpoint == "" {
		return fmt.Errorf("covert: endpoint is empty")
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 30 * time.Second
	}

	log.Printf("[covert] starting HTTPS outbound channel to %s", opts.Endpoint)

	// Start background disk queue processor
	go processDiskQueue(ctx, opts)

	// Optionally start a metrics HTTP server
	if opts.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		srv := &http.Server{Addr: opts.MetricsAddr, Handler: mux}
		go func() {
			<-ctx.Done()
			srv.Close()
		}()
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[covert] metrics server error: %v", err)
			}
		}()
		log.Printf("[covert] metrics server listening on %s", opts.MetricsAddr)
	}

	client := &http.Client{Timeout: 20 * time.Second}

	// Simple heartbeat + send loop. For now, we block here for the lifetime
	// of the process; a future change could expose a cancelable context.
	ticker := time.NewTicker(opts.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// send heartbeat
			p := payload{
				Pairing: opts.Token,
				Type:    "heartbeat",
				Time:    time.Now().Unix(),
			}
			if err := postJSON(client, opts.Endpoint, p); err != nil {
				log.Printf("[covert] heartbeat failed: %v", err)
				// simple backoff sleep to avoid busy loops
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// SendData performs a synchronous POST of captured data. Useful for one-off
// events like session logs. Returns an error on failure.
func SendData(endpoint, pairing string, data []byte) error {
	// First try to enqueue to the active websocket connection if present.
	p := payload{
		Pairing: pairing,
		Type:    "data",
		Time:    time.Now().Unix(),
		Data:    string(data),
	}
	wsMu.RLock()
	ch := wsMsgCh
	wsMu.RUnlock()
	if ch != nil {
		select {
		case ch <- p:
			return nil
		default:
			// channel full: fall back to HTTP POST to avoid dropping silently
			log.Printf("[covert] ws queue full, falling back to HTTP POST")
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if err := postJSON(client, endpoint, p); err != nil {
		metricSendFailure.Inc()
		// As a last resort, enqueue to disk for later retry
		log.Printf("[covert] HTTP POST failed, enqueuing to disk: %v", err)
		if e := enqueueToDisk(p, queueDir, 0); e != nil {
			return fmt.Errorf("post error: %v; enqueue error: %v", err, e)
		}
		return err
	}
	metricSendSuccess.Inc()
	return nil
}

func postJSON(client *http.Client, endpoint string, p payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

// StartWSS connects to the specified WSS endpoint, authenticates using a
// simple JSON auth message containing the pairing token, and maintains a
// heartbeat. It will reconnect with exponential backoff on failures and
// returns only if the caller cancels or an unrecoverable error occurs.
func StartWSS(ctx context.Context, opts Options) error {
	log.Printf("[covert] starting WSS channel to %s", opts.Endpoint)

	// Prepare TLS config with optional custom CA bundle
	tlsConfig := &tls.Config{}
	if opts.CABundlePath != "" {
		data, err := ioutil.ReadFile(opts.CABundlePath)
		if err != nil {
			return fmt.Errorf("failed to read CA bundle: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return fmt.Errorf("failed to parse CA bundle PEM")
		}
		tlsConfig.RootCAs = pool
	}

	// start disk queue processor
	go processDiskQueue(ctx, opts)

	backoff := 1 * time.Second
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		dialer := websocket.Dialer{TLSClientConfig: tlsConfig}
		conn, resp, err := dialer.Dial(opts.Endpoint, nil)
		if err != nil {
			if resp != nil {
				log.Printf("[covert] dial error status=%s err=%v", resp.Status, err)
			} else {
				log.Printf("[covert] dial error: %v", err)
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			continue
		}

		// Reset backoff on successful connect
		backoff = 1 * time.Second
		log.Printf("[covert] wss connected to %s", opts.Endpoint)

		// Send auth message
		auth := payload{Pairing: opts.Token, Type: "auth", Time: time.Now().Unix()}
		if err := conn.WriteJSON(auth); err != nil {
			log.Printf("[covert] auth write failed: %v", err)
			conn.Close()
			continue
		}

		// Create outbound message channel and writer goroutine for this connection
		wsMu.Lock()
		wsMsgCh = make(chan payload, 512)
		wsMu.Unlock()

		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)
			for msg := range wsMsgCh {
				if err := conn.WriteJSON(msg); err != nil {
					log.Printf("[covert] ws write error: %v", err)
					metricWSSSendFailure.Inc()
					return
				}
			}
		}()

		// Start heartbeat ticker and read loop
		ticker := time.NewTicker(opts.HeartbeatInterval)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				var msg payload
				if err := conn.ReadJSON(&msg); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						log.Printf("[covert] wss closed normally: %v", err)
					} else {
						log.Printf("[covert] wss read error: %v", err)
					}
					return
				}
				log.Printf("[covert] received message type=%s", msg.Type)
			}
		}()

		// heartbeat loop
		for {
			select {
			case <-ctx.Done():
				// close outbound writer channel, wait for it to finish
				wsMu.Lock()
				if wsMsgCh != nil {
					close(wsMsgCh)
					wsMsgCh = nil
				}
				wsMu.Unlock()
				<-writerDone
				conn.Close()
				return nil
			case <-ticker.C:
				hb := payload{Pairing: opts.Token, Type: "heartbeat", Time: time.Now().Unix()}
				if err := conn.WriteJSON(hb); err != nil {
					log.Printf("[covert] heartbeat write failed: %v", err)
					conn.Close()
					ticker.Stop()
					<-done
					break
				}
			case <-done:
				ticker.Stop()
				// normal read loop termination
				// close outbound writer channel, wait for it to finish
				wsMu.Lock()
				close(wsMsgCh)
				wsMsgCh = nil
				wsMu.Unlock()
				<-writerDone
				conn.Close()
				break
			}
		}
		// if we get here we will loop and reconnect
	}
}
