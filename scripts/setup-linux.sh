#!/usr/bin/env bash
set -euo pipefail

echo "Setting up proto-honeypot dependencies on Linux..."
# Install Rust via rustup if missing
if ! command -v cargo >/dev/null 2>&1; then
  echo "rustup/cargo not found. Installing rustup..."
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
  source "$HOME/.cargo/env"
fi

# Ensure required system packages (Debian/Ubuntu example)
if command -v apt-get >/dev/null 2>&1; then
  sudo apt-get update
  sudo apt-get install -y build-essential pkg-config libssl-dev
fi

echo "Build the project (release)..."
cargo build --release

echo "Done. Run with: ./target/release/proto-honeypot --config config.toml"
