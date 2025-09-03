#!/usr/bin/env bash
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <pending_file> <base64_key>"
  exit 2
fi
fpath=$1
key_b64=$2
python3 scripts/decrypt_pending.py "$fpath" "$key_b64"
