# PowerShell setup script for proto-honeypot (Windows)
Write-Host "Ensure you have Rust toolchain installed via rustup (https://rustup.rs)"
Write-Host "If you plan to build native deps, install Visual Studio Build Tools (C++ workloads)."

Write-Host "Building project..."
cargo build --release
Write-Host "Done. Run with: .\\target\\release\\proto-honeypot.exe --config config.toml"
