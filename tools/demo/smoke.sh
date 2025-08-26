#!/usr/bin/env bash
set -euo pipefail

# Build demo
echo "Building demo..."
go build -o tools/demo/demo tools/demo/demo.go

echo "Starting demo in background..."
./tools/demo/demo &
DEMO_PID=$!
sleep 1

METRICS_URL="http://localhost:9090/metrics"
echo "Fetching metrics from $METRICS_URL"
if curl -fsS "$METRICS_URL" | grep -q "zecx_covert_queue_enqueued_total"; then
  echo "Metrics present: OK"
else
  echo "Metrics missing or demo not running"
  kill $DEMO_PID || true
  exit 2
fi

echo "Stopping demo (pid=$DEMO_PID)"
kill $DEMO_PID || true
wait $DEMO_PID 2>/dev/null || true
rm -f tools/demo/demo
echo "Done."
