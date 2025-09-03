use anyhow::{Context, Result};
use clap::Parser;
use rand::RngCore;
use base64::{engine::general_purpose, Engine as _};
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;
use uuid::Uuid;

#[derive(Parser, Debug)]
#[command(author, version, about = "proto-honeypot: lightweight async honeypot")]
pub struct Cli {
    /// Path to config file (TOML)
    #[arg(long)]
    pub config: Option<PathBuf>,

    /// Backend URL to POST logs to (overrides config)
    #[arg(long)]
    pub backend_url: Option<String>,

    /// Honeypot root path
    #[arg(long)]
    pub honeypot_root: Option<PathBuf>,

    /// Apply iptables rules
    #[arg(long)]
    pub apply_iptables: bool,
    /// Backend bearer token for authenticated POSTs
    #[arg(long)]
    pub backend_token: Option<String>,
    /// Run recovery: resend any pending files and exit
    #[arg(long)]
    pub resend_pending: bool,
    /// List configured TCP & UDP ports (after config expansion) and exit
    #[arg(long)]
    pub list_ports: bool,
    /// Metrics / health server bind address (host:port). If unset, disabled.
    #[arg(long)]
    pub metrics_addr: Option<String>,
    /// Log format: text or json (default text)
    #[arg(long)]
    pub log_format: Option<String>,
}

#[derive(Deserialize, Debug)]
pub struct FileConfig {
    pub backend_url: Option<String>,
    pub honeypot_root: Option<String>,
    pub listen_addr: Option<String>,
    pub report_interval_seconds: Option<u64>,
    pub apply_iptables: Option<bool>,
    pub encrypt_logs: Option<bool>,
    pub encryption_key_base64: Option<String>,
    pub udp_ports: Option<Vec<u16>>,
    pub backend_token: Option<String>,
    pub max_pending_files: Option<usize>,
    pub metrics_addr: Option<String>,
    pub log_format: Option<String>,
}

#[derive(Clone, Debug)]
pub struct Config {
    pub backend_url: Option<String>,
    pub honeypot_root: PathBuf,
    pub listen_addr: String,
    pub report_interval_seconds: u64,
    pub apply_iptables: bool,
    pub pairing_code: String,
    pub ports: Vec<u16>,
    pub encrypt_logs: bool,
    pub encryption_key: Option<Vec<u8>>,
    pub backend_token: Option<String>,
    pub udp_ports: Vec<u16>,
    pub max_pending_files: usize,
    pub metrics_addr: Option<String>,
    pub log_format: String,
}

impl Config {
    pub async fn from_cli(cli: &Cli) -> Result<Self> {
        // Load file config: explicit --config, otherwise auto-detect ./config.toml if present
        let file_cfg: Option<FileConfig> = if let Some(path) = &cli.config {
            let s = fs::read_to_string(path)
                .with_context(|| format!("reading config {}", path.display()))?;
            Some(toml::from_str(&s).with_context(|| "parsing config file")?)
        } else {
            let default_path = PathBuf::from("./config.toml");
            if default_path.exists() {
                let s = fs::read_to_string(&default_path)
                    .with_context(|| format!("reading config {}", default_path.display()))?;
                Some(toml::from_str(&s).with_context(|| "parsing config file")?)
            } else {
                // First run experience: create a default config.toml
                let template = r#"# proto-honeypot configuration
listen_addr = "0.0.0.0"
udp_ports = [5353]
# Expose metrics to both WSL and Windows
metrics_addr = "0.0.0.0:9100"
# No remote backend by default
backend_url = ""
report_interval_seconds = 60
apply_iptables = false
log_format = "text"
# Optional: set to true and provide encryption_key_base64 to encrypt pending logs
encrypt_logs = false
max_pending_files = 100
"#;
                let _ = fs::write(&default_path, template);
                None
            }
        };
        // Backend URL: CLI overrides file
        let backend_url = cli
            .backend_url
            .clone()
            .or_else(|| file_cfg.as_ref().and_then(|f| f.backend_url.clone()));

        let honeypot_root = cli
            .honeypot_root
            .clone()
            .or_else(|| file_cfg.as_ref().and_then(|f| f.honeypot_root.clone().map(PathBuf::from)))
            .unwrap_or_else(|| PathBuf::from("./honeypot_fs"));

        let listen_addr = file_cfg
            .as_ref()
            .and_then(|f| f.listen_addr.clone())
            .unwrap_or_else(|| "0.0.0.0".to_string());

        let report_interval_seconds = file_cfg
            .as_ref()
            .and_then(|f| f.report_interval_seconds)
            .unwrap_or(30);

        let apply_iptables = cli.apply_iptables
            || file_cfg.as_ref().and_then(|f| f.apply_iptables).unwrap_or(false);

        let encrypt_logs = file_cfg.as_ref().and_then(|f| f.encrypt_logs).unwrap_or(false);
        let encryption_key = file_cfg
            .as_ref()
            .and_then(|f| f.encryption_key_base64.clone())
            .or_else(|| cli.config.as_ref().and_then(|_| None))
            .and_then(|s| general_purpose::STANDARD.decode(s).ok());

        let backend_token = cli.backend_token.clone().or_else(|| file_cfg.as_ref().and_then(|f| f.backend_token.clone()));

        // Default port list: common + tricky + ephemeral
        let mut ports = vec![22, 23, 21, 25, 80, 443, 3306, 8080, 1337, 2222, 31337, 4444, 5555, 6969];
        ports.extend(49152..=49160);

        // Add some random ephemeral ports (e.g., 5 ports)
        let mut rng = rand::thread_rng();
        for _ in 0..5 {
            let p = 1025u16 + (rng.next_u32() % (65535 - 1025)) as u16;
            if !ports.contains(&p) {
                ports.push(p);
            }
        }

        // Generate a cryptographically secure pairing code (UUID v4)
        let pairing_code = Uuid::new_v4().to_string();

        // UDP ports
        let udp_ports = file_cfg.as_ref().and_then(|f| f.udp_ports.clone()).unwrap_or_else(|| vec![5353]);
    let max_pending_files = file_cfg.as_ref().and_then(|f| f.max_pending_files).unwrap_or(100);
    let metrics_addr = cli.metrics_addr.clone()
        .or_else(|| file_cfg.as_ref().and_then(|f| f.metrics_addr.clone()));
    let log_format = cli.log_format.clone()
        .or_else(|| file_cfg.as_ref().and_then(|f| f.log_format.clone()))
        .unwrap_or_else(|| "text".to_string());

    // Basic validation (deduplicate ports, ensure non-empty)
    let mut ports_set = std::collections::BTreeSet::new();
    let mut dedup_ports = Vec::new();
    for p in ports { if ports_set.insert(p) { dedup_ports.push(p); } }
    if dedup_ports.is_empty() { anyhow::bail!("No TCP ports configured after deduplication"); }
    let mut udp_set = std::collections::BTreeSet::new();
    let mut dedup_udp = Vec::new();
    for p in udp_ports { if udp_set.insert(p) { dedup_udp.push(p); } }
    if dedup_udp.is_empty() { anyhow::bail!("No UDP ports configured after deduplication"); }

        Ok(Config {
            backend_url,
            honeypot_root,
            listen_addr,
            report_interval_seconds,
            apply_iptables,
            pairing_code,
            ports: dedup_ports,
            encrypt_logs,
            encryption_key,
            backend_token,
            udp_ports: dedup_udp,
            max_pending_files,
            metrics_addr,
            log_format,
        })
    }
}

// Internal convenience builder (exposed for integration tests)
impl Config {
    pub fn test_builder() -> TestConfigBuilder { TestConfigBuilder::default() }
}

#[derive(Default)]
#[doc(hidden)]
pub struct TestConfigBuilder {
    backend_url: Option<String>,
    honeypot_root: Option<PathBuf>,
    listen_addr: Option<String>,
    report_interval_seconds: Option<u64>,
    apply_iptables: bool,
    pairing_code: Option<String>,
    ports: Option<Vec<u16>>,
    encrypt_logs: bool,
    encryption_key: Option<Vec<u8>>,
    backend_token: Option<String>,
    udp_ports: Option<Vec<u16>>,
    max_pending_files: Option<usize>,
    metrics_addr: Option<String>,
    log_format: Option<String>,
}

impl TestConfigBuilder {
    pub fn backend_url(mut self, v: Option<String>) -> Self { self.backend_url = v; self }
    pub fn honeypot_root<P: Into<PathBuf>>(mut self, p: P) -> Self { self.honeypot_root = Some(p.into()); self }
    pub fn listen_addr<S: Into<String>>(mut self, s: S) -> Self { self.listen_addr = Some(s.into()); self }
    pub fn report_interval_seconds(mut self, v: u64) -> Self { self.report_interval_seconds = Some(v); self }
    pub fn apply_iptables(mut self, v: bool) -> Self { self.apply_iptables = v; self }
    pub fn pairing_code<S: Into<String>>(mut self, s: S) -> Self { self.pairing_code = Some(s.into()); self }
    pub fn ports(mut self, v: Vec<u16>) -> Self { self.ports = Some(v); self }
    pub fn encrypt_logs(mut self, v: bool) -> Self { self.encrypt_logs = v; self }
    pub fn encryption_key(mut self, v: Option<Vec<u8>>) -> Self { self.encryption_key = v; self }
    pub fn backend_token(mut self, v: Option<String>) -> Self { self.backend_token = v; self }
    pub fn udp_ports(mut self, v: Vec<u16>) -> Self { self.udp_ports = Some(v); self }
    pub fn max_pending_files(mut self, v: usize) -> Self { self.max_pending_files = Some(v); self }
    pub fn metrics_addr<S: Into<String>>(mut self, s: S) -> Self { self.metrics_addr = Some(s.into()); self }
    pub fn log_format<S: Into<String>>(mut self, s: S) -> Self { self.log_format = Some(s.into()); self }
    pub fn build(self) -> Config {
        Config {
            backend_url: self.backend_url,
            honeypot_root: self.honeypot_root.unwrap_or_else(|| PathBuf::from("./honeypot_fs")),
            listen_addr: self.listen_addr.unwrap_or_else(|| "127.0.0.1".into()),
            report_interval_seconds: self.report_interval_seconds.unwrap_or(30),
            apply_iptables: self.apply_iptables,
            pairing_code: self.pairing_code.unwrap_or_else(|| "test-pairing".into()),
            ports: self.ports.unwrap_or_else(|| vec![22,80]),
            encrypt_logs: self.encrypt_logs,
            encryption_key: self.encryption_key,
            backend_token: self.backend_token,
            udp_ports: self.udp_ports.unwrap_or_else(|| vec![5353]),
            max_pending_files: self.max_pending_files.unwrap_or(100),
            metrics_addr: self.metrics_addr,
            log_format: self.log_format.unwrap_or_else(|| "text".into()),
        }
    }
}
