#!/usr/bin/env bash
# build.sh — build the wt binary into ./bin (override with BIN_DIR).
set -euo pipefail

cd "$(dirname "$0")"

BIN_DIR="${BIN_DIR:-bin}"
OUT="$BIN_DIR/wt"

mkdir -p "$BIN_DIR"
go build -trimpath -o "$OUT" .

echo "Built $OUT"
