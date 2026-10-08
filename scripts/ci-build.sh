#!/usr/bin/env bash
# Build Wozzle's Windows binaries for CI: embedded frontend + three exes.
# Kept as a single script because some runners (e.g. Gitee Go) split multi
# command lines into independent statements, which breaks && chains and
# inline env assignments.
set -euo pipefail

export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
export CGO_ENABLED=0

# --- node + pnpm for the embedded frontend ---
if ! command -v node >/dev/null 2>&1; then
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y nodejs
fi
if ! command -v pnpm >/dev/null 2>&1; then
  npm install -g pnpm@10 --registry=https://registry.npmmirror.com
fi

# --- frontend ---
(cd web && pnpm install --frozen-lockfile && pnpm build)

# --- Windows binaries (all-Go deps cross-compile cleanly from Linux) ---
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o wozzle.exe .
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o wozzle-console.exe .
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o wozzle-mcp.exe ./cmd/wozzle-mcp

ls -lh wozzle*.exe
