package covert

import (
    "io/ioutil"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestEnqueueToDiskMetrics(t *testing.T) {
    // prepare temp queue dir
    dir, err := ioutil.TempDir("", "zecx-metric-test")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(dir)

    // record current metrics
    before := testutil.ToFloat64(metricQueueEnqueued)

    p := payload{Pairing: "p", Type: "data", Time: 1, Data: "x"}
    if err := enqueueToDisk(p, dir, 0); err != nil {
        t.Fatalf("enqueue failed: %v", err)
    }

    after := testutil.ToFloat64(metricQueueEnqueued)
    if after-before < 1 {
        t.Fatalf("expected enqueued counter to increase; before=%v after=%v", before, after)
    }

    // queue depth gauge should reflect 1 file
    files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
    if len(files) != 1 {
        t.Fatalf("expected one queued file, found %d", len(files))
    }
    depth := testutil.ToFloat64(metricQueueDepth)
    if int(depth) < 1 {
        t.Fatalf("expected queue depth >=1, got %v", depth)
    }
}

func TestSendDataMetricsSuccess(t *testing.T) {
    // start a test HTTP server that returns 200
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))
    defer srv.Close()

    before := testutil.ToFloat64(metricSendSuccess)
    if err := SendData(srv.URL, "p", []byte("ok")); err != nil {
        t.Fatalf("SendData error: %v", err)
    }
    after := testutil.ToFloat64(metricSendSuccess)
    if after-before < 1 {
        t.Fatalf("expected send_success to increase; before=%v after=%v", before, after)
    }
}

func TestSendDataMetricsFailure(t *testing.T) {
    // server returns 500
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(500)
    }))
    defer srv.Close()

    // ensure queue dir is clean
    _ = os.RemoveAll(queueDir)
    defer os.RemoveAll(queueDir)

    before := testutil.ToFloat64(metricSendFailure)
    _ = SendData(srv.URL, "p", []byte("bad"))
    after := testutil.ToFloat64(metricSendFailure)
    if after-before < 1 {
        t.Fatalf("expected send_failure to increase; before=%v after=%v", before, after)
    }
}
