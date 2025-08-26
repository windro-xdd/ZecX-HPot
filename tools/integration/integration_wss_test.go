package integration

import (
    "context"
    "io/ioutil"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
    "strings"
    "os"

    "github.com/gorilla/websocket"
    "zecx-deploy/internal/covert"
)

var upgrader = websocket.Upgrader{}

func TestWSSPath(t *testing.T) {
    receivedCh := make(chan map[string]interface{}, 1)

    // WSS-like handler (ws over http for testing)
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/wss" {
            http.NotFound(w, r)
            return
        }
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            t.Logf("upgrade error: %v", err)
            return
        }
        defer conn.Close()
        // read auth
        var auth map[string]interface{}
        if err := conn.ReadJSON(&auth); err != nil {
            t.Logf("read auth error: %v", err)
            return
        }
        if auth["type"] != "auth" {
            t.Logf("unexpected auth message: %v", auth)
        }
        // read next data message
        var msg map[string]interface{}
        if err := conn.ReadJSON(&msg); err != nil {
            t.Logf("read msg error: %v", err)
            return
        }
        receivedCh <- msg
        // keep connection open briefly
        time.Sleep(100 * time.Millisecond)
    }))
    defer srv.Close()

    // convert http://... to ws://...
    wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/wss"

    qdir, _ := ioutil.TempDir("", "zecx-queue-wss")
    defer func() { _ = os.RemoveAll(qdir) }()

    opts := covert.Options{
        Endpoint:          wsURL,
        Token:             "wss-test-pair",
        HeartbeatInterval: 2 * time.Second,
        QueueDir:          qdir,
        MaxQueueBytes:     1024 * 1024,
    }

    // start WSS client loop with context
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go func() {
        if err := covert.StartWSS(ctx, opts); err != nil {
            t.Logf("StartWSS exited: %v", err)
        }
    }()

    // wait for connection setup
    time.Sleep(200 * time.Millisecond)

    // send data (should prefer ws path)
    payload := []byte("wss-hello")
    if err := covert.SendData(opts.Endpoint, opts.Token, payload); err != nil {
        t.Fatalf("SendData failed: %v", err)
    }

    select {
    case got := <-receivedCh:
        if got["type"] != "data" {
            t.Fatalf("expected data message, got: %v", got)
        }
        if got["data"] != string(payload) {
            t.Fatalf("data mismatch: got=%v want=%s", got["data"], string(payload))
        }
    case <-time.After(2 * time.Second):
        t.Fatalf("timeout waiting for ws message")
    }
    // cancel background WSS loop
    cancel()
}
