#!/usr/bin/env bash
# build.sh — build wt (native) and wt.exe (windows/amd64) into ./bin and
# materialize the shell cd-wrappers next to them. Override the output
# directory with BIN_DIR.
set -euo pipefail

cd "$(dirname "$0")"

BIN_DIR="${BIN_DIR:-bin}"

mkdir -p "$BIN_DIR"

# On both platforms the real binary is wt.bin[.exe] and the `wt` entry point
# is a wrapper script. On Windows that is load-bearing (cmd.exe prefers .exe
# over .cmd in a directory, so a wt.exe would shadow the wt.cmd wrapper and
# typing `wt` could never cd); on Linux it keeps the layout symmetric while
# the cd itself comes from the shell function (`wt shell-init`).
go build -trimpath -o "$BIN_DIR/wt.bin" .
GOOS=windows GOARCH=amd64 go build -trimpath -o "$BIN_DIR/wt.bin.exe" .

cp shell/wt "$BIN_DIR/wt"
chmod +x "$BIN_DIR/wt"
cp shell/wt.sh "$BIN_DIR/wt.sh"
cp shell/wt.cmd "$BIN_DIR/wt.cmd"
# Drop artifacts of older layouts that would shadow the wrappers.
rm -f "$BIN_DIR/wt.exe" "$BIN_DIR/w.cmd"

echo "Built $BIN_DIR/wt.bin and $BIN_DIR/wt.bin.exe (entry points: wt, wt.cmd)"
