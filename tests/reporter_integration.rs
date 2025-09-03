use proto_honeypot::config::Config;
use proto_honeypot::reporter::Reporter;
use proto_honeypot::listener::ConnLog;
use std::sync::Arc;
use tokio::sync::RwLock;
use std::net::TcpListener;
use std::thread;

#[tokio::test]
async fn reporter_posts_to_backend() {
    // start a simple TCP server to accept one HTTP POST
    let listener = TcpListener::bind("127.0.0.1:18080").expect("bind");
    thread::spawn(move || {
        if let Ok((mut stream, _)) = listener.accept() {
            use std::io::{Read, Write};
            let mut buf = [0u8; 4096];
            let n = stream.read(&mut buf).unwrap_or(0);
            let req = String::from_utf8_lossy(&buf[..n]).to_string();
            // respond 200 OK
            let resp = "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK";
            let _ = stream.write_all(resp.as_bytes());
            // ensure request contained pairing_code and logs
            assert!(req.contains("pairing_code"));
            assert!(req.contains("logs"));
        }
    });

    // build a minimal config pointing to our mock server
    let cfg = Config::test_builder()
        .backend_url(Some("http://127.0.0.1:18080/".into()))
        .listen_addr("0.0.0.0")
        .report_interval_seconds(1)
        .ports(vec![80])
        .udp_ports(vec![5353])
        .build();

    let shared = Arc::new(RwLock::new(Vec::new()));
    let (_tx, rx) = tokio::sync::broadcast::channel(1);
    let reporter = Reporter::new(cfg, shared.clone(), rx, None);

    // create a fake log and send once
    let logs = vec![ConnLog { timestamp: "t".to_string(), src_ip: "1.2.3.4".to_string(), src_port: 1234, dst_port: 80, protocol: "tcp".to_string(), commands: vec!["GET /".to_string()] }];
    let ok = reporter.send_once(logs).await;
    assert!(ok, "reporter failed to send payload");
}
