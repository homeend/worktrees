#!/usr/bin/env bash
# build-worktrees.sh — build the full-name, self-installing binary into ./bin:
# worktrees (native) and worktrees.exe (windows/amd64). Copy one anywhere and
# run it once — it materializes the wt entry points (wt.bin[.exe] + wt/wt.cmd)
# next to itself, same as a `go install` binary does. Override the output
# directory with BIN_DIR.
set -euo pipefail

cd "$(dirname "$0")"

BIN_DIR="${BIN_DIR:-bin}"

mkdir -p "$BIN_DIR"
go build -trimpath -o "$BIN_DIR/worktrees" .
GOOS=windows GOARCH=amd64 go build -trimpath -o "$BIN_DIR/worktrees.exe" .

echo "Built $BIN_DIR/worktrees and $BIN_DIR/worktrees.exe (self-installing)"
