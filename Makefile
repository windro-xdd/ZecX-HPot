# Makefile for proto-honeypot - automation helpers

.PHONY: all build test run resend decrypt setup-linux

all: build

build:
	cargo build --release

test:
	cargo test -- --nocapture

run:
	cargo run --release -- --config config.toml

resend:
	cargo run --release -- --resend-pending --config config.toml

decrypt:
	python3 scripts/decrypt_pending.py

setup-linux:
	bash scripts/setup-linux.sh

install: build
	@echo "Install binary to /usr/local/bin and systemd unit (requires sudo)"
	sudo cp target/release/proto-honeypot /usr/local/bin/
	sudo cp systemd/proto-honeypot.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable --now proto-honeypot

generate-key:
	@echo "Generating a 32-byte base64 key"
	head -c32 /dev/urandom | base64 -w0

decrypt-sh:
	python3 scripts/decrypt_pending.py
