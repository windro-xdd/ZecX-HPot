use proto_honeypot::config::Config;
use proto_honeypot::fsgen;
use std::fs;
use uuid::Uuid;

#[tokio::test]
async fn fsgen_creates_expected_files() {
    // Create a unique temporary honeypot root
    let dir = std::env::temp_dir().join(format!("proto_honeypot_test_{}", Uuid::new_v4()));
    if dir.exists() { let _ = fs::remove_dir_all(&dir); }
    fs::create_dir_all(&dir).expect("create temp dir");

    let cfg = Config::test_builder()
        .honeypot_root(dir.clone())
        .listen_addr("127.0.0.1")
        .ports(vec![22,80])
        .udp_ports(vec![5353])
        .build();

    // Run generator
    fsgen::create_and_populate(&cfg).await.expect("fsgen");

    // Check a few expected files
    assert!(dir.join("etc/passwd").exists(), "etc/passwd missing");
    assert!(dir.join("etc/ssh/sshd_config").exists(), "sshd_config missing");
    assert!(dir.join("var/log/auth.log").exists(), "auth.log missing");
    assert!(dir.join("home/user/.ssh/authorized_keys").exists(), "authorized_keys missing");

    // Clean up
    let _ = fs::remove_dir_all(&dir);
}
