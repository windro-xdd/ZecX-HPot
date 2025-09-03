//! Network listener: binds many TCP & UDP ports, emits basic banners / minimal protocol
//! emulation, stores interactions in an in-memory buffer, and (optionally) records metrics.

use crate::config::Config;
use crate::metrics::{record_tcp_conn, record_udp_msg, record_bytes, Metrics};
use anyhow::Result;
use serde::{Serialize, Deserialize};
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::{RwLock, broadcast};
use tracing::{info, warn, error};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ConnLog {
    pub timestamp: String,
    pub src_ip: String,
    pub src_port: u16,
    pub dst_port: u16,
    pub protocol: String,
    pub commands: Vec<String>,
}

pub struct HoneypotListener {
    cfg: Config,
    shared_logs: Arc<RwLock<Vec<ConnLog>>>,
    shutdown: broadcast::Receiver<()>,
    metrics: Option<Arc<Metrics>>,
}

impl HoneypotListener {
    pub fn new(
        cfg: Config,
        shared_logs: Arc<RwLock<Vec<ConnLog>>>,
        shutdown: broadcast::Receiver<()>,
        metrics: Option<Arc<Metrics>>,
    ) -> Self {
        Self { cfg, shared_logs, shutdown, metrics }
    }

    pub async fn run(&self) -> Result<()> {
        let mut handles = Vec::new();

        // UDP listeners
        for up in self.cfg.udp_ports.clone() {
            let shared = self.shared_logs.clone();
            let mut shutdown_rx = self.shutdown.resubscribe();
            let metrics = self.metrics.clone();
            let addr = format!("0.0.0.0:{}", up);
            info!(protocol="udp", %addr, "listening");
            handles.push(tokio::spawn(async move {
                use tokio::net::UdpSocket;
                if let Ok(sock) = UdpSocket::bind(&addr).await {
                    let mut buf = [0u8; 2048];
                    loop {
                        tokio::select! {
                            _ = shutdown_rx.recv() => break,
                            res = sock.recv_from(&mut buf) => match res {
                                Ok((n, peer)) => {
                                    if n == 0 { continue; }
                                    record_udp_msg(&metrics); record_bytes(&metrics, n);
                                    let s = String::from_utf8_lossy(&buf[..n]).to_string();
                                    let log = ConnLog {
                                        timestamp: chrono::Utc::now().to_rfc3339(),
                                        src_ip: peer.ip().to_string(),
                                        src_port: peer.port(),
                                        dst_port: up,
                                        protocol: "udp".into(),
                                        commands: vec![s],
                                    };
                                    let mut w = shared.write().await; w.push(log);
                                }
                                Err(_) => break,
                            }
                        }
                    }
                }
            }));
        }

        // TCP listeners
        for port in &self.cfg.ports {
            let addr = format!("{}:{}", self.cfg.listen_addr, port);
            let listener = match TcpListener::bind(&addr).await {
                Ok(l) => l,
                Err(e) => { warn!(%addr, error=%e, "failed to bind TCP port; skipping"); continue; }
            };
            info!(protocol="tcp", %addr, "listening");
            let shared = self.shared_logs.clone();
            let mut shutdown_rx = self.shutdown.resubscribe();
            let metrics = self.metrics.clone();
            let p = *port;
            handles.push(tokio::spawn(async move {
                loop {
                    tokio::select! {
                        _ = shutdown_rx.recv() => break,
                        res = listener.accept() => match res {
                            Ok((stream, peer)) => {
                                let shared_i = shared.clone();
                                let metrics_i = metrics.clone();
                                tokio::spawn(async move {
                                    if let Err(e) = handle_conn(stream, peer, p, shared_i, metrics_i).await { error!(port=p, error=?e, "connection handler error"); }
                                });
                            }
                            Err(e) => { error!(port=p, error=?e, "accept error"); break; }
                        }
                    }
                }
            }));
        }

        for h in handles { let _ = h.await; }
        Ok(())
    }
}

async fn handle_conn(
    mut stream: TcpStream,
    peer: SocketAddr,
    dst_port: u16,
    shared: Arc<RwLock<Vec<ConnLog>>>,
    metrics: Option<Arc<Metrics>>,
) -> Result<()> {
    record_tcp_conn(&metrics);
    let banner = banner_for_port(dst_port);
    if !banner.is_empty() {
        let _ = stream.write_all(banner.as_bytes()).await;
        record_bytes(&metrics, banner.len());
    }
    let mut buf = [0u8; 1024];
    let mut commands = Vec::new();

    if dst_port == 21 { // FTP minimal dialogue
        loop {
            let res = tokio::time::timeout(std::time::Duration::from_secs(30), stream.read(&mut buf)).await;
            if let Ok(Ok(0)) = res { break; }
            else if let Ok(Ok(n)) = res {
                record_bytes(&metrics, n);
                let s = String::from_utf8_lossy(&buf[..n]).to_string();
                for line in s.lines(){
                    let cmd = line.trim();
                    if cmd.is_empty(){ continue; }
                    commands.push(cmd.to_string());
                    let up = cmd.to_uppercase();
                    if up.starts_with("USER ") { let _=stream.write_all(b"331 Please specify the password.\r\n").await; }
                    else if up.starts_with("PASS ") { let _=stream.write_all(b"230 Login successful.\r\n").await; }
                    else if up.starts_with("QUIT") { let _=stream.write_all(b"221 Goodbye.\r\n").await; break; }
                    else { let _=stream.write_all(b"500 Unknown command.\r\n").await; }
                }
            }
            else { break; }
            if commands.len()>50 { break; }
        }
    } else if dst_port == 22 { // SSH-ish prompt
        let res = tokio::time::timeout(std::time::Duration::from_secs(30), stream.read(&mut buf)).await;
        if let Ok(Ok(n)) = res { if n>0 {
            record_bytes(&metrics, n);
            let s=String::from_utf8_lossy(&buf[..n]).to_string();
            for line in s.lines(){ if !line.trim().is_empty(){ commands.push(line.trim().to_string()); } }
            let _=stream.write_all(b"Password: ").await;
            if let Ok(Ok(m))=tokio::time::timeout(std::time::Duration::from_secs(15), stream.read(&mut buf)).await {
                if m>0 { record_bytes(&metrics, m); let _=stream.write_all(b"Permission denied, please try again.\r\n").await; }
            }
        } }
    } else { // generic / HTTP-aware
        loop {
            match tokio::time::timeout(std::time::Duration::from_secs(5), stream.read(&mut buf)).await {
                Ok(Ok(0)) => break,
                Ok(Ok(n)) => {
                    if n==0 { break; }
                    record_bytes(&metrics, n);
                    let s=String::from_utf8_lossy(&buf[..n]).to_string();
                    if (dst_port==80 || dst_port==8080) && s.starts_with("GET ") {
                        let resp = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: 70\r\n\r\n<html><body><h1>Welcome</h1><p>Apache/2.4.29 (Ubuntu)</p></body></html>";
                        let _=stream.write_all(resp.as_bytes()).await; record_bytes(&metrics, resp.len());
                        if let Some(line)=s.lines().next(){ commands.push(line.trim().to_string()); }
                        break;
                    }
                    for line in s.lines(){ if !line.trim().is_empty(){ commands.push(line.trim().to_string()); } }
                    let _=stream.write_all(b"\n").await;
                }
                _ => break,
            }
            if commands.len()>50 { break; }
        }
    }

    let log = ConnLog {
        timestamp: chrono::Utc::now().to_rfc3339(),
        src_ip: peer.ip().to_string(),
        src_port: peer.port(),
        dst_port,
        protocol: "tcp".into(),
        commands,
    };
    let mut w = shared.write().await; w.push(log);
    Ok(())
}

fn banner_for_port(port: u16) -> String {
    match port {
        22 => "SSH-2.0-OpenSSH_7.4p1 Debian-10\r\n".into(),
        23 => "Welcome to Telnet\r\n".into(),
        21 => "220 FTP server (vsftpd 3.0.3)\r\n".into(),
        25 => "220 SMTP Fake Mail Server\r\n".into(),
        80 => "HTTP/1.1 200 OK\r\nServer: nginx/1.14.0\r\n\r\n".into(),
        443 => "HTTP/1.1 200 OK\r\nServer: Apache/2.4.29 (Ubuntu)\r\n\r\n".into(),
        3306 => "\x00\x00\x00\x0a5.7.31-log\r\n".into(),
        8080 => "HTTP/1.1 200 OK\r\nServer: Jetty(9.x)\r\n\r\n".into(),
        1337 => "Leet service v1.0\r\n".into(),
        2222 => "SSH-2.0-OpenSSH_8.0\r\n".into(),
        31337 => "Elite service ready\r\n".into(),
        4444 | 5555 | 6969 => "Generic remote service\r\n".into(),
        p if (49152..=49160).contains(&p) => "UPnP/SSDP-like response\r\n".into(),
        _ => "".into(),
    }
}
