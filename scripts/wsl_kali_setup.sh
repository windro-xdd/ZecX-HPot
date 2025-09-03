#!/usr/bin/env bash
# Automated setup & smoke test for proto-honeypot on Kali / Debian-based WSL
# Usage: bash scripts/wsl_kali_setup.sh
set -euo pipefail

echo "[1/8] Updating APT indexes" && sudo apt update -y

echo "[2/8] Installing base packages" && sudo apt install -y --no-install-recommends \
  build-essential pkg-config libssl-dev curl git netcat-openbsd socat ca-certificates

if ! command -v rustc >/dev/null 2>&1; then
  echo "[3/8] Installing Rust toolchain (stable)"
  curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal
  source "$HOME/.cargo/env"
else
  echo "[3/8] Rust already installed: $(rustc --version)"
fi

echo "[4/8] Running cargo test" && cargo test --quiet

echo "[5/8] Building release binary" && cargo build --release

if [ ! -f config.toml ]; then
  echo "[6/8] Creating config.toml from example" && cp config.example.toml config.toml
fi

echo "[7/8] Generating sample encryption key (optional)" 
KEY=$(head -c32 /dev/urandom | base64)
echo "# Example generated key (do NOT commit):" >> config.toml
echo "# encryption_key_base64 = \"$KEY\"" >> config.toml

echo "[8/8] Quick smoke: starting honeypot in background" 
./target/release/proto-honeypot --config config.toml &
HP_PID=$!
# Give it a moment
sleep 1

# Pick an available high port from the config (attempt 8080)
if command -v curl >/dev/null 2>&1; then
  echo "GET /" | nc 127.0.0.1 8080 || true
fi

echo "Sending UDP probe" && echo PING | nc -u 127.0.0.1 5353 || true

sleep 1

if [ -d honeypot_fs/logs ]; then
  echo "Recent pending files:" && ls -1 honeypot_fs/logs | tail -5 || true
fi

echo "Stopping honeypot (PID $HP_PID)" && kill $HP_PID 2>/dev/null || true
wait $HP_PID 2>/dev/null || true

echo "Setup & smoke test complete."
