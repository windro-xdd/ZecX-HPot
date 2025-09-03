use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use tokio::sync::RwLock;
use crate::listener::ConnLog;
use std::net::SocketAddr;
use tokio::io::AsyncWriteExt;
use tracing::info;

#[derive(Default)]
pub struct Metrics {
    pub tcp_connections: AtomicU64,
    pub udp_messages: AtomicU64,
    pub bytes_received: AtomicU64,
    pub logs_buffered: AtomicU64,
}

impl Metrics {
    pub fn inc_tcp(&self) { self.tcp_connections.fetch_add(1, Ordering::Relaxed); }
    pub fn inc_udp(&self) { self.udp_messages.fetch_add(1, Ordering::Relaxed); }
    pub fn add_bytes(&self, n: u64) { self.bytes_received.fetch_add(n, Ordering::Relaxed); }
    pub fn set_buffered(&self, n: u64) { self.logs_buffered.store(n, Ordering::Relaxed); }
}

pub async fn spawn_metrics_server(addr: String, metrics: Arc<Metrics>, shared_logs: Arc<RwLock<Vec<ConnLog>>>) {
    let bind: SocketAddr = addr.parse().expect("invalid metrics bind address");
    info!(%bind, "metrics server starting");
    tokio::spawn(async move {
        use tokio::net::TcpListener;
        let listener = match TcpListener::bind(bind).await { Ok(l)=>l, Err(e)=> { tracing::warn!(error=?e, "metrics bind failed"); return; } };
        loop {
            if let Ok((mut s, _peer)) = listener.accept().await {
                let m = &metrics;
                // refresh buffered gauge
                let len = shared_logs.read().await.len() as u64;
                m.set_buffered(len);
                let body = format!("# HELP tcp_connections total TCP connections handled\n# TYPE tcp_connections counter\ntcp_connections {}\n# HELP udp_messages total UDP datagrams received\n# TYPE udp_messages counter\nudp_messages {}\n# HELP bytes_received total bytes received across TCP/UDP\n# TYPE bytes_received counter\nbytes_received {}\n# HELP logs_buffered current number of logs in memory buffer\n# TYPE logs_buffered gauge\nlogs_buffered {}\n", m.tcp_connections.load(Ordering::Relaxed), m.udp_messages.load(Ordering::Relaxed), m.bytes_received.load(Ordering::Relaxed), m.logs_buffered.load(Ordering::Relaxed));
                let resp = format!("HTTP/1.1 200 OK\r\nContent-Type: text/plain; version=0.0.4\r\nContent-Length: {}\r\n\r\n{}", body.len(), body);
                let _ = s.write_all(resp.as_bytes()).await;
            }
        }
    });
}

pub fn record_tcp_conn(metrics: &Option<Arc<Metrics>>) { if let Some(m) = metrics { m.inc_tcp(); } }
pub fn record_udp_msg(metrics: &Option<Arc<Metrics>>) { if let Some(m) = metrics { m.inc_udp(); } }
pub fn record_bytes(metrics: &Option<Arc<Metrics>>, n: usize) { if let Some(m) = metrics { m.add_bytes(n as u64); } }