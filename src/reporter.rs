//! Reporter: periodically takes collected logs and POSTs them to backend in JSON format
//! with pairing code and exponential backoff retries.

use crate::config::Config;
use crate::listener::ConnLog;
use anyhow::Result;
use reqwest::Client;
use serde::Serialize;
use std::sync::Arc;
use tokio::sync::{RwLock, broadcast};
use std::path::PathBuf;
use std::fs;
use aes_gcm::{Aes256Gcm, Nonce, KeyInit}; // AES-GCM requires 12-byte nonce
use aes_gcm::aead::Aead;
use rand_core::{OsRng, RngCore};
use base64::{engine::general_purpose, Engine as _};
use chrono::Utc;
use tracing::{info, warn, debug};

#[derive(Serialize)]
struct Payload<'a> {
    pairing_code: &'a str,
    logs: &'a [ConnLog],
}

pub struct Reporter {
    cfg: Config,
    shared_logs: Arc<RwLock<Vec<ConnLog>>>,
    client: Client,
    shutdown: broadcast::Receiver<()>,
    #[allow(dead_code)]
    metrics: Option<Arc<crate::metrics::Metrics>>,
}

impl Reporter {
    pub fn new(cfg: Config, shared_logs: Arc<RwLock<Vec<ConnLog>>>, shutdown: broadcast::Receiver<()>, metrics: Option<Arc<crate::metrics::Metrics>>) -> Self {
        let client = Client::builder().build().unwrap();
        Self { cfg, shared_logs, client, shutdown, metrics }
    }

    fn persist_pending_logs(&self, logs: &[ConnLog], pending_path: &PathBuf) {
        if let Ok(s) = serde_json::to_string(logs) {
            if self.cfg.encrypt_logs {
                if let Some(key) = &self.cfg.encryption_key {
                    if key.len() == 32 {
                        let cipher = match Aes256Gcm::new_from_slice(&key) {
                            Ok(c) => c,
                            Err(_) => {
                                let _ = fs::write(&pending_path, s);
                                return;
                            }
                        };
                        let mut nonce_bytes = [0u8; 12];
                        OsRng.try_fill_bytes(&mut nonce_bytes).ok();
                        let nonce = Nonce::from_slice(&nonce_bytes);
                        match cipher.encrypt(nonce, s.as_bytes()) {
                            Ok(ct) => {
                                // store nonce + ct, base64
                                let mut out = Vec::new();
                                out.extend_from_slice(&nonce_bytes);
                                out.extend_from_slice(&ct);
                                let b64 = general_purpose::STANDARD.encode(&out);
                                let _ = fs::write(&pending_path, b64);
                            }
                            Err(_) => {
                                let _ = fs::write(&pending_path, s);
                            }
                        }
                    } else {
                        let _ = fs::write(&pending_path, s);
                    }
                } else {
                    let _ = fs::write(&pending_path, s);
                }
            } else {
                let _ = fs::write(&pending_path, s);
            }
        }
    }

    pub async fn run(&mut self) -> Result<()> {
    // On startup, attempt to recover and resend any pending files left on disk
    self.recover_pending_files().await;

        loop {
            tokio::select! {
                _ = self.shutdown.recv() => { info!("reporter shutdown"); return Ok(()); }
                _ = tokio::time::sleep(std::time::Duration::from_secs(self.cfg.report_interval_seconds)) => {}
            }

            // prepare payload
            let mut logs = Vec::new();
            {
                let mut w = self.shared_logs.write().await;
                if w.is_empty() { continue; }
                logs.append(&mut *w);
            }
            // persist pending payload to disk before sending (rotated file)
            let logs_dir = self.cfg.honeypot_root.join("logs");
            let _ = fs::create_dir_all(&logs_dir);
            self.prune_old_pending_files(&logs_dir, true);
            let file_name = format!("pending_{}.json", Utc::now().timestamp());
            let pending_path: PathBuf = logs_dir.join(&file_name);
            self.persist_pending_logs(&logs, &pending_path);

            let url = match &self.cfg.backend_url {
                Some(u) => u.clone(),
                None => {
                    warn!("no backend URL configured; skipping report");
                    continue;
                }
            };

            let payload = Payload { pairing_code: &self.cfg.pairing_code, logs: &logs };

            // Attempt send with retries
            let sent = self.send_payload_with_retry(&url, &payload).await;
            if sent {
                // remove pending file
                let _ = fs::remove_file(&pending_path);
                debug!("report delivered and pending file removed");
            } else {
                warn!("failed to deliver logs after retries; logs remain on disk");
                // Put logs back into memory buffer for immediate visibility
                let mut w = self.shared_logs.write().await;
                w.append(&mut logs);
            }
        }
    }

    /// Prune oldest pending_*.json files to enforce retention. If account_new is true,
    /// we prune assuming one more file will soon be added.
    pub fn prune_old_pending_files(&self, logs_dir: &PathBuf, account_new: bool) {
        if let Ok(entries) = fs::read_dir(&logs_dir) {
            let mut files: Vec<_> = entries.flatten().filter_map(|e| {
                let p = e.path();
                if let Some(name) = p.file_name().and_then(|n| n.to_str()) {
                    if name.starts_with("pending_") && name.ends_with(".json") {
                        let md = e.metadata().ok()?;
                        let modified = md.modified().ok()?;
                        return Some((p, modified));
                    }
                }
                None
            }).collect();
            files.sort_by_key(|(_, m)| *m);
            let count = files.len();
            let limit = self.cfg.max_pending_files;
            if count > 0 && (count >= limit || (account_new && count + 1 > limit)) {
                let excess = if account_new { count + 1 - limit } else { count - limit };
                if excess > 0 {
                    for (p, _) in files.into_iter().take(excess) { let _ = fs::remove_file(p); }
                }
            }
        }
    }

    async fn recover_pending_files(&self) {
        // Check honeypot_root/pending_report.json
        let mut candidates: Vec<PathBuf> = Vec::new();
        let pending_report = self.cfg.honeypot_root.join("pending_report.json");
        if pending_report.exists() { candidates.push(pending_report); }
        // Check logs directory for pending_*.json
        let logs_dir = self.cfg.honeypot_root.join("logs");
        if let Ok(entries) = fs::read_dir(&logs_dir) {
            for e in entries.flatten() {
                let p = e.path();
                if let Some(name) = p.file_name().and_then(|n| n.to_str()) {
                    if name.starts_with("pending_") && name.ends_with(".json") {
                        candidates.push(p.clone());
                    }
                }
            }
        }

        for path in candidates {
            if let Ok(s) = fs::read_to_string(&path) {
                // try decode base64; if decode fails, assume plaintext JSON
                let logs_bytes = if self.cfg.encrypt_logs {
                    if let Some(key) = &self.cfg.encryption_key {
                        if key.len() == 32 {
                            if let Ok(raw) = general_purpose::STANDARD.decode(s.trim()) {
                                if raw.len() > 12 {
                                    let (nonce_bytes, ct) = raw.split_at(12);
                                    if let Ok(cipher) = Aes256Gcm::new_from_slice(&key) {
                                        let nonce = Nonce::from_slice(nonce_bytes);
                                        if let Ok(pt) = cipher.decrypt(nonce, ct) {
                                            pt
                                        } else {
                                            // fallback to plaintext bytes
                                            s.into_bytes()
                                        }
                                    } else {
                                        s.into_bytes()
                                    }
                                } else { s.into_bytes() }
                            } else { s.into_bytes() }
                        } else { s.into_bytes() }
                    } else { s.into_bytes() }
                } else { s.into_bytes() };

                // try parse logs JSON
                if let Ok(logs) = serde_json::from_slice::<Vec<ConnLog>>(&logs_bytes) {
                    let payload = Payload { pairing_code: &self.cfg.pairing_code, logs: &logs };
                    let url = match &self.cfg.backend_url {
                        Some(u) => u.clone(),
                        None => { continue; }
                    };
                    let sent = self.send_payload_with_retry(&url, &payload).await;
                    if sent {
                        let _ = fs::remove_file(&path);
                    }
                }
            }
        }
    }

    /// Public helper to run pending-file recovery once (used by CLI `--resend-pending`).
    pub async fn recover_pending_files_once(&self) {
        self.recover_pending_files().await;
    }

    async fn send_payload_with_retry(&self, url: &str, payload: &Payload<'_>) -> bool {
        let mut backoff = 1u64;
        let max_retries = 5;
        for _attempt in 0..=max_retries {
            let mut req = self.client.post(url).json(payload);
            if let Some(token) = &self.cfg.backend_token {
                req = req.bearer_auth(token);
            }
            let res = req.send().await;
            match res {
                Ok(r) if r.status().is_success() => { return true; }
                Ok(r) => { warn!(status=?r.status(), "report failed status"); }
                Err(e) => { warn!(error=?e, "report POST error"); }
            }
            tokio::time::sleep(std::time::Duration::from_secs(backoff)).await;
            backoff = std::cmp::min(backoff * 2, 60);
        }
        false
    }

    // New helper for tests: send a single payload (used by integration test)
    pub async fn send_once(&self, logs: Vec<ConnLog>) -> bool {
    // Always persist logs first so tests can inspect the file even without a backend
    let pending_path: PathBuf = self.cfg.honeypot_root.join("pending_report.json");
    self.persist_pending_logs(&logs, &pending_path);

    // If no backend URL configured, treat persistence as success (skip network)
    let Some(url) = &self.cfg.backend_url else { return true; };

    let payload = Payload { pairing_code: &self.cfg.pairing_code, logs: &logs };
    let ok = self.send_payload_with_retry(url, &payload).await;
    if ok { let _ = fs::remove_file(&pending_path); }
    ok
    }
}
