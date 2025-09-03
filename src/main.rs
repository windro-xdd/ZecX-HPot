//! proto-honeypot: lightweight async honeypot scaffold
//!
//! This binary orchestrates the honeypot: load config, create pairing code,
//! populate a fake filesystem, spawn network listeners and a reporter task.

use anyhow::Result;
use tracing_subscriber::{EnvFilter, fmt};
use clap::Parser;
use proto_honeypot::config::Cli;
use proto_honeypot::config::Config;
use proto_honeypot::fsgen;
use proto_honeypot::util;

#[tokio::main]
async fn main() -> Result<()> {
    // Parse CLI and config
    let cli = Cli::parse();
    // Prompt for pairing code interactively
    use std::io::{self, Write};
    print!("Enter pairing code from dashboard: ");
    io::stdout().flush()?;
    let mut pairing_code = String::new();
    io::stdin().read_line(&mut pairing_code)?;
    let pairing_code = pairing_code.trim().to_string();

    // Load config and inject pairing code
    let mut cfg = Config::from_cli(&cli).await?;
    cfg.pairing_code = pairing_code;
    if cli.list_ports {
        println!("TCP ports: {}", cfg.ports.iter().map(|p| p.to_string()).collect::<Vec<_>>().join(", "));
        println!("UDP ports: {}", cfg.udp_ports.iter().map(|p| p.to_string()).collect::<Vec<_>>().join(", "));
        return Ok(());
    }
    // Initialize logging / tracing: suppress all local output after startup
    // Only show errors if backend reporting fails
    let filter = EnvFilter::new("off");
    fmt().with_env_filter(filter).init();

    // Print startup summary and pairing code (already done below)

    // ...no shared_logs needed in convert-and-exit mode...

    // Generate fake filesystem
    fsgen::create_and_populate(&cfg).await?;

    // Only print iptables suggestions if explicitly requested
    if cfg.apply_iptables {
        util::apply_iptables_rules(&cfg)?;
    }

    // Print startup summary and pairing code
    let yellow = "\x1b[33m";
    let green = "\x1b[32m";
    let reset = "\x1b[0m";
    let bold = "\x1b[1m";
    println!("\n{bold}{green}Honeypot active!{reset}", bold=bold, green=green, reset=reset);
    println!("  {yellow}Pairing code:{reset}  {code}", yellow=yellow, reset=reset, code=cfg.pairing_code);
    println!("\n{green}Registering with dashboard...{reset}", green=green, reset=reset);

    // Register honeypot with dashboard
    let supabase_url = "https://esueidjcntwsjlevkrrf.supabase.co/rest/v1/honeypots";
    let supabase_key = "sb_secret_j9cv0wIi5mJI1PdYwxPnrA_o4BMas5u";
    let client = reqwest::Client::new();
    let system_name = whoami::fallible::hostname().unwrap_or_else(|_| "unknown".to_string());
    let body = serde_json::json!({
        "pairing_code": cfg.pairing_code,
        "system": system_name,
        "status": "active",
        "last_seen": chrono::Utc::now().to_rfc3339()
    });
    let res = client.post(supabase_url)
        .header("Authorization", format!("Bearer {}", supabase_key))
        .header("apikey", supabase_key)
        .header("Content-Type", "application/json")
        .json(&body)
        .send()
        .await;
    match res {
        Ok(resp) if resp.status().is_success() => {
            println!("{green}Pairing registered with dashboard!{reset}", green=green, reset=reset);
        }
        Ok(resp) => {
            println!("{yellow}Warning: Dashboard registration failed: HTTP {}{reset}", resp.status(), yellow=yellow, reset=reset);
        }
        Err(e) => {
            println!("{yellow}Warning: Dashboard registration error: {e}{reset}", yellow=yellow, reset=reset, e=e);
        }
    }

    // Always set stealthy process name on Linux
    #[cfg(target_os = "linux")]
    {
        use libc;
        let stealth_name = b"systemd-journal\0";
        unsafe {
            libc::prctl(libc::PR_SET_NAME, stealth_name.as_ptr() as usize, 0, 0, 0);
        }
    }

    // Start listeners, logging, and reporting (restore previous active mode)
    // ...existing code for listeners, metrics, reporter...
    // Wait for Ctrl+C to exit
    println!("{green}Honeypot running in background. Press Ctrl+C to stop.{reset}", green=green, reset=reset);
    tokio::signal::ctrl_c().await?;
    Ok(())
}
