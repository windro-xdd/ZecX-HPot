package integration

import (
    "context"
    "io/ioutil"
    "net"
    "net/http"
    "strings"
    "testing"
    "time"

    "zecx-deploy/internal/covert"
)

// TestMetricsSmoke starts StartTunnel with a dynamic metrics address and
// verifies that the /metrics endpoint serves the expected Prometheus metrics.
func TestMetricsSmoke(t *testing.T) {
    // pick a free port
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("listen failed: %v", err)
    }
    addr := ln.Addr().String()
    ln.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    opts := covert.Options{
        Endpoint:          "http://127.0.0.1:1/hook", // dummy
        Token:             "smoke",
        HeartbeatInterval: 2 * time.Second,
        QueueDir:          "",
        MaxQueueBytes:     0,
        MetricsAddr:       addr,
    }

    go func() {
        if err := covert.StartTunnel(ctx, opts); err != nil {
            t.Logf("StartTunnel exited: %v", err)
        }
    }()

    // wait briefly for server to start
    time.Sleep(200 * time.Millisecond)

    resp, err := http.Get("http://" + addr + "/metrics")
    if err != nil {
        t.Fatalf("failed to GET metrics: %v", err)
    }
    defer resp.Body.Close()
    b, _ := ioutil.ReadAll(resp.Body)
    s := string(b)
    if !strings.Contains(s, "zecx_covert_queue_enqueued_total") {
        t.Fatalf("metrics endpoint missing expected metric; got: %s", s)
    }

    cancel()
}
