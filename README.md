# proto-honeypot

Lightweight async honeypot scaffold in Rust (Linux-only prototype). Creates a fake filesystem, listens on many ports,
serves deceptive banners, logs connections, and posts logs to a backend.

Build

```powershell
cargo build --release
```

Run

```powershell
cargo run -- --backend-url https://example.com/api/logs
```

Configuration

You can provide a TOML config file via `--config` or create `~/.config/proto-honeypot/config.toml`.

Example `config.toml` is provided as `config.example.toml`.

Encryption

To encrypt pending logs at rest, set `encrypt_logs = true` and provide a base64-encoded 32-byte key in `encryption_key_base64`.

Generate a 32-byte key on PowerShell and print base64:

```powershell
[System.Convert]::ToBase64String((1..32 | ForEach-Object { [byte] (Get-Random -Maximum 256) }) )
```

Place the resulting string in `encryption_key_base64` in your config.

UDP Ports

The honeypot can also bind lightweight UDP listeners (default: 5353). Configure via `udp_ports` in the TOML. Each UDP datagram is recorded as a log entry with protocol `udp` and its payload captured (UTF-8 lossily decoded).

Testing Helpers

For internal tests a `Config::test_builder()` helper exists to construct configs without manually specifying every field. This keeps tests resilient when new fields are added.

Pending / Encrypted Log Files

Before sending, a JSON array of `ConnLog` entries is persisted in `honeypot_root/logs/pending_<ts>.json` (or `pending_report.json` for single send). If encryption is enabled each file contains base64 of: 12-byte nonce || ciphertext (AES-256-GCM). Decrypt by base64-decoding, splitting the first 12 bytes as nonce, then decrypting the remainder with the 32-byte key.

Prototype Quickstart

1. Copy `config.example.toml` to `config.toml` and adjust `backend_url` (or leave unset to just collect locally).
2. (Optional) Generate an encryption key and enable `encrypt_logs`.
3. Build & run:

```powershell
cargo build --release
./target/release/proto-honeypot --config config.toml
```

4. Trigger activity (locally):
	- `curl http://127.0.0.1:<some_open_port>/` for HTTP banner
	- `nc 127.0.0.1 <port>` then send lines
	- `echo "PING" | nc -u 127.0.0.1 5353` for UDP test

5. Inspect pending logs in `honeypot_fs/logs/` or encrypted `pending_report.json` if you used `send_once` logic.
6. List resolved port sets without starting listeners:
```bash
./target/release/proto-honeypot --config config.toml --list-ports
```

Resend Pending Only

If the process crashed and pending files remain, run:

```powershell
./target/release/proto-honeypot --config config.toml --resend-pending
```

License

MIT License (see `LICENSE`).

Logging
Set verbosity: `RUST_LOG=debug` (e.g. `RUST_LOG=proto_honeypot=debug,info`). Example:
```bash
RUST_LOG=debug ./target/release/proto-honeypot --config config.toml
```
Ctrl+C triggers graceful shutdown signaling all listener/report tasks.

Container (Docker) Usage

Build image:
```bash
docker build -t proto-honeypot:latest .
```
Run (detached):
```bash
docker run -d --name ph -p 2222:22 -p 8080:8080 proto-honeypot:latest
```

Packaging (Linux Tarball)

On Linux:
```bash
chmod +x scripts/package-linux.sh
scripts/package-linux.sh
ls dist/*.tar.gz
```
Resulting archive includes binary, example config, systemd unit, and license.
