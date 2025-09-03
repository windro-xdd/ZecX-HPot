# Multi-stage build for proto-honeypot (Linux x86_64)
FROM rust:1.81 as builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
# Cache deps
RUN mkdir src && echo "fn main(){}" > src/main.rs && cargo build --release || true
COPY src ./src
COPY config.example.toml README.md PHASES.md ./
RUN cargo build --release

FROM debian:stable-slim
WORKDIR /opt/proto-honeypot
RUN useradd -r -s /usr/sbin/nologin honeypot && mkdir -p /var/lib/proto-honeypot /etc/proto-honeypot && chown -R honeypot:honeypot /var/lib/proto-honeypot
COPY --from=builder /app/target/release/proto-honeypot /usr/local/bin/proto-honeypot
COPY config.example.toml /etc/proto-honeypot/config.toml
USER honeypot
EXPOSE 22 23 21 25 80 443 3306 8080
ENTRYPOINT ["/usr/local/bin/proto-honeypot", "--config", "/etc/proto-honeypot/config.toml"]
