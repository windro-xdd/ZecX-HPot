#!/usr/bin/env bash
set -euo pipefail
crate=proto-honeypot
version=$(grep '^version' Cargo.toml | head -1 | cut -d '"' -f2)
arch=$(uname -m)
out_dir=dist
rm -rf "$out_dir" && mkdir -p "$out_dir/$crate-$version-linux-$arch"

echo "Building release binary..."
cargo build --release
cp target/release/$crate "$out_dir/$crate-$version-linux-$arch/"
cp README.md LICENSE config.example.toml "$out_dir/$crate-$version-linux-$arch/"
cp -r systemd "$out_dir/$crate-$version-linux-$arch/systemd"

tar -C "$out_dir" -czf "$out_dir/$crate-$version-linux-$arch.tar.gz" "$crate-$version-linux-$arch"
echo "Created $out_dir/$crate-$version-linux-$arch.tar.gz"
