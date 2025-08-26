package integration

import (
    "context"
    "encoding/json"
    "io/ioutil"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "time"

    "zecx-deploy/internal/covert"
)

// TestEndToEnd starts a local HTTP collector and verifies that SendData
// delivers a payload (using HTTP POST path). This is a lightweight
// integration test that doesn't spawn external binaries.
func TestEndToEnd(t *testing.T) {
    // placeholder channel removed; not needed for this test

    // Simple collector that records the last POST body
    var lastBody []byte
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            t.Fatalf("expected POST, got %s", r.Method)
        }
        b, _ := ioutil.ReadAll(r.Body)
        lastBody = b
        w.WriteHeader(200)
    }))
    defer srv.Close()

    // create a temp queue dir
    qdir, _ := ioutil.TempDir("", "zecx-queue-test")
    defer os.RemoveAll(qdir)

    opts := covert.Options{
        Endpoint:          srv.URL,
        Token:             "testpair",
        HeartbeatInterval: 2 * time.Second,
        QueueDir:          qdir,
        MaxQueueBytes:     1024 * 1024,
    }

    // start disk queue processor in background (it will no-op until we enqueue)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go covert.StartTunnel(ctx, opts)

    // Give tunnel a moment to start
    time.Sleep(200 * time.Millisecond)

    // send a test payload
    data := []byte("hello-integration")
    if err := covert.SendData(opts.Endpoint, opts.Token, data); err != nil {
        t.Fatalf("SendData failed: %v", err)
    }

    // wait for the collector to receive it
    time.Sleep(200 * time.Millisecond)

    if len(lastBody) == 0 {
        t.Fatalf("collector didn't receive payload; lastBody empty")
    }
    var payload map[string]interface{}
    if err := json.Unmarshal(lastBody, &payload); err != nil {
        t.Fatalf("invalid json received: %v", err)
    }
    if payload["pairing_code"] != opts.Token {
        t.Fatalf("unexpected pairing code: %v", payload["pairing_code"])
    }
}
