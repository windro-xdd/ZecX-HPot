#!/bin/bash
# reset_honeypot.sh - Remove all honeypot traces and restore system to normal

set -e

# Remove honeypot filesystem and config
echo "Removing honeypot_fs and config.toml..."
rm -rf ./honeypot_fs ./config.toml

# Remove honeypot binary (optional)
echo "Removing honeypot binary..."
rm -f ./target/release/proto-honeypot

# Revert iptables rules for all honeypot ports
echo "Reverting iptables rules for honeypot ports..."
for port in 22 23 21 25 80 443 3306 8080 1337 2222 31337 4444 5555 6969 49152 49153 49154 49155 49156 49157 49158 49159 49160; do
    sudo iptables -D INPUT -p tcp --dport $port -j ACCEPT 2>/dev/null || true
done
for port in 5353; do
    sudo iptables -D INPUT -p udp --dport $port -j ACCEPT 2>/dev/null || true
done


echo "System reset: honeypot files, config, and binary removed."
