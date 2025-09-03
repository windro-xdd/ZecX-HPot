use proto_honeypot::config::Config;
use proto_honeypot::reporter::Reporter;
use proto_honeypot::listener::ConnLog;
use std::sync::Arc;
use tokio::sync::RwLock;
use std::fs;

#[tokio::test]
async fn retention_prunes_old_pending() {
    let tmp = std::env::temp_dir().join("ph_retention_test");
    let _ = fs::remove_dir_all(&tmp);
    fs::create_dir_all(&tmp).unwrap();
    let logs_dir = tmp.join("logs");
    fs::create_dir_all(&logs_dir).unwrap();

    // config with small max_pending_files
    let cfg = Config::test_builder()
        .honeypot_root(tmp.clone())
        .max_pending_files(3)
        .build();
    let shared = Arc::new(RwLock::new(Vec::<ConnLog>::new()));
    let (_tx, rx) = tokio::sync::broadcast::channel(1);
    let reporter = Reporter::new(cfg.clone(), shared.clone(), rx, None);

    // Create 5 fake pending files with increasing timestamps
    for i in 0..5 {
        let f = logs_dir.join(format!("pending_{}.json", 1000 + i));
        fs::write(&f, "[]").unwrap();
        // Ensure mtime is increasing (cannot set directly portable; rely on sequential creation order)
        std::thread::sleep(std::time::Duration::from_millis(5));
    }
    // prune (not accounting new) to enforce limit 3
    reporter.prune_old_pending_files(&logs_dir, false);

    let files: Vec<_> = fs::read_dir(&logs_dir).unwrap().flatten().map(|e| e.path()).collect();
    assert!(files.len() <= 3, "expected <=3 files after pruning, found {}", files.len());

    // Clean up
    let _ = fs::remove_dir_all(&tmp);
}
