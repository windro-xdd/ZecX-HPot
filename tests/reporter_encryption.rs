use proto_honeypot::config::Config;
use proto_honeypot::reporter::Reporter;
use proto_honeypot::listener::ConnLog;
use std::sync::Arc;
use tokio::sync::RwLock;
use rand_core::OsRng;
use rand_core::RngCore;
use base64::{engine::general_purpose, Engine as _};
use aes_gcm::{Aes256Gcm, Nonce, KeyInit, aead::Aead};
use std::fs;

#[tokio::test]
async fn reporter_encrypted_persist_and_decrypt() {
    // Generate a random 32-byte key
    let mut key_bytes = [0u8; 32];
    OsRng.fill_bytes(&mut key_bytes);
    let _key_b64 = general_purpose::STANDARD.encode(&key_bytes);

    // Setup config
    let root = std::env::temp_dir().join("proto_honeypot_enc_test");
    let _ = fs::remove_dir_all(&root);
    fs::create_dir_all(&root).unwrap();
    let logs_dir = root.join("logs");
    fs::create_dir_all(&logs_dir).unwrap();
    let cfg = Config::test_builder()
        // No backend_url so send_once will persist and return immediately (faster test)
        .backend_url(None)
        .honeypot_root(root.clone())
        .listen_addr("127.0.0.1")
        .report_interval_seconds(1)
        .ports(vec![22])
        .udp_ports(vec![5353])
        .encrypt_logs(true)
        .encryption_key(Some(key_bytes.to_vec()))
        .build();
    let shared = Arc::new(RwLock::new(Vec::new()));
    let (_tx, rx) = tokio::sync::broadcast::channel(1);
    let reporter = Reporter::new(cfg.clone(), shared.clone(), rx, None);

    // Add a log entry
    let log = ConnLog {
        timestamp: "now".to_string(),
        src_ip: "1.2.3.4".to_string(),
        src_port: 1234,
        dst_port: 22,
        protocol: "tcp".to_string(),
        commands: vec!["USER root".to_string()],
    };
    {
        let mut w = shared.write().await;
        w.push(log.clone());
    }

    // Run one report interval (simulate)
    // We'll call the persist logic directly by running the first part of run()
    // (simulate up to file write)
    // Instead, call send_once, which will persist and try to send (but backend_url is None)
    let _ = reporter.send_once(vec![log.clone()]).await;

    // Find the pending file (send_once writes to honeypot_root/pending_report.json)
    let pending = root.join("pending_report.json");
    assert!(pending.exists(), "pending_report.json not found");
    let b64 = fs::read_to_string(&pending).unwrap();
    let raw = general_purpose::STANDARD.decode(&b64).unwrap();
    assert!(raw.len() > 12, "encrypted file too short");
    let (nonce_bytes, ct) = raw.split_at(12);
    let cipher = Aes256Gcm::new_from_slice(&key_bytes).unwrap();
    let nonce = Nonce::from_slice(nonce_bytes);
    let pt = cipher.decrypt(nonce, ct).expect("decrypt");
    let json = String::from_utf8(pt).unwrap();
    assert!(json.contains("USER root"), "decrypted log missing expected content");
    // Clean up
    let _ = fs::remove_dir_all(&root);
}
