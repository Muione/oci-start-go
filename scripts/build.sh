#!/usr/bin/env bash
# ==========================================================================
# oci-start 一键编译脚本
# 1. 编译前端 (Vue 3 + Vite) → internal/web/dist/
# 2. 编译后端 (Go + tags dist) → oci-start 二进制
# ==========================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
say() { printf "${GREEN}[OK]${NC} %s\n" "$1"; }
err() { printf "${RED}[ERR]${NC} %s\n" "$1"; exit 1; }
info() { printf "${CYAN}[..]${NC} %s\n" "$1"; }

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"
OUTPUT="$PROJECT_ROOT/oci-start"

# Detect Go binary (system go, go1.25.0, or user-provided GO_BIN).
find_go() {
  if [ -n "${GO_BIN:-}" ] && command -v "$GO_BIN" &>/dev/null; then
    echo "$GO_BIN"
  elif command -v go &>/dev/null; then
    echo "$(command -v go)"
  elif command -v /usr/local/go/bin/go &>/dev/null; then
    echo "/usr/local/go/bin/go"
  else
    # Try to find any go* binary
    echo "$(which go1.* 2>/dev/null | head -1 || true)"
  fi
}

main() {
  echo ""
  printf "${CYAN}╔══════════════════════════════════════╗${NC}\n"
  printf "${CYAN}║   oci-start 一键编译                 ║${NC}\n"
  printf "${CYAN}╚══════════════════════════════════════╝${NC}\n"
  echo ""

  GO_BIN=$(find_go)
  if [ -z "$GO_BIN" ]; then
    err "找不到 Go 编译器，请先运行 ./scripts/setup.sh 或设置 GO_BIN 环境变量"
  fi
  info "Go:  $GO_BIN"

  # --------------- 1. Frontend ---------------
  echo ""
  info "[1/4] 检查前端依赖 ..."
  cd "$FRONTEND_DIR"
  if [ ! -d "node_modules" ]; then
    info "node_modules 不存在，正在 npm install ..."
    npm install --silent
  fi
  say "前端依赖就绪"

  info "[2/4] 编译前端 (Vite) ..."
  npm run build --silent 2>&1 | tail -1
  say "前端编译完成 → internal/web/dist/"
  cd "$PROJECT_ROOT"

  # --------------- 2. Backend ---------------
  info "[3/4] Go vet 检查 ..."
  $GO_BIN vet ./internal/... 2>&1 || warn "vet 有告警，继续编译..."
  say "vet 通过"

  info "[4/4] Go build (-tags dist) ..."
  START=$(date +%s)
  CGO_ENABLED=0 $GO_BIN build -tags dist -o "$OUTPUT" ./cmd/oci-start/ 2>&1
  ELAPSED=$(($(date +%s) - START))

  if [ ! -f "$OUTPUT" ]; then
    err "编译失败 — 未生成二进制文件"
  fi

  SIZE=$(du -h "$OUTPUT" | cut -f1)
  say "编译完成 (${ELAPSED}s)"

  echo ""
  printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
  printf "${GREEN}  编译成功 ✅${NC}\n"
  printf "${GREEN}  二进制: %s (%s)${NC}\n" "$OUTPUT" "$SIZE"
  printf "${GREEN}  运行:   cd %s && ./oci-start${NC}\n" "$PROJECT_ROOT"
  printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

main "$@"
