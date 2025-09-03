#!/usr/bin/env python3
"""Decrypt a pending encrypted file created by proto-honeypot.
Usage: python3 decrypt_pending.py <pending_file> <base64_key>
"""
import sys, base64
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

if len(sys.argv) < 3:
    print("Usage: decrypt_pending.py <pending_file> <base64_key>")
    sys.exit(2)

fpath = sys.argv[1]
key_b64 = sys.argv[2]
raw_b64 = open(fpath, 'rb').read().strip()
raw = base64.b64decode(raw_b64)
nonce = raw[:12]
ct = raw[12:]
key = base64.b64decode(key_b64)
pt = AESGCM(key).decrypt(nonce, ct, None)
print(pt.decode('utf-8'))
