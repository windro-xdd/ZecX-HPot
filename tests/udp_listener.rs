use proto_honeypot::config::Config;
use proto_honeypot::listener::{HoneypotListener, ConnLog};
use std::sync::Arc;
use tokio::sync::RwLock;

#[tokio::test]
async fn udp_listener_captures_datagram() {
    let cfg = Config::test_builder()
        .listen_addr("127.0.0.1")
        .udp_ports(vec![0]) // use 0 then replace with assigned port? We'll just pick 53535 ephemeral
        .build();
    // override the udp port to a high ephemeral to avoid conflicts
    let mut cfg = cfg; // make mutable
    cfg.udp_ports = vec![53535];

    let shared = Arc::new(RwLock::new(Vec::<ConnLog>::new()));
    let (_tx, rx) = tokio::sync::broadcast::channel(1);
    let listener = HoneypotListener::new(cfg.clone(), shared.clone(), rx, None);
    let handle = tokio::spawn(async move { let _ = listener.run().await; });

    // give listener time to bind
    tokio::time::sleep(std::time::Duration::from_millis(200)).await;

    // send a datagram
    use tokio::net::UdpSocket;
    let sock = UdpSocket::bind("127.0.0.1:0").await.unwrap();
    let _ = sock.send_to(b"PING", ("127.0.0.1", 53535)).await.unwrap();

    tokio::time::sleep(std::time::Duration::from_millis(200)).await;

    let logs = shared.read().await;
    assert!(logs.iter().any(|l| l.protocol=="udp" && l.dst_port==53535), "UDP log not captured");

    // stop test (listener task will end when test ends)
    drop(handle);
}
