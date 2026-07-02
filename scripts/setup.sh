#!/usr/bin/env bash
# ==========================================================================
# oci-start 环境准备脚本
# 检查并安装 Go / Node.js / npm 依赖，一键完成开发环境初始化。
# ==========================================================================
set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
say() { printf "${GREEN}[OK]${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$1"; }
err()  { printf "${RED}[ERR]${NC} %s\n" "$1"; exit 1; }
info() { printf "${CYAN}[..]${NC} %s\n" "$1"; }

# --------------- config ---------------
GO_MIN="1.25"
NODE_MIN="18"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

# --------------- check go ---------------
check_go() {
  info "检查 Go ..."
  if command -v go &>/dev/null; then
    local ver
    ver=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    say "Go $ver 已安装"
    return 0
  fi
  if command -v go1.25.0 &>/dev/null; then
    say "go1.25.0 可用"
    return 0
  fi
  warn "Go 未安装（需要 >= $GO_MIN）"
  echo "  安装方法: https://go.dev/dl/ 或 snap install go --classic"
  return 1
}

# --------------- check node ---------------
check_node() {
  info "检查 Node.js ..."
  if ! command -v node &>/dev/null; then
    warn "Node.js 未安装（需要 >= $NODE_MIN）"
    echo "  安装方法: curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt-get install -y nodejs"
    return 1
  fi
  local ver
  ver=$(node -v | grep -oP '\d+' | head -1)
  say "Node.js v$(node -v) 已安装"
}

check_npm() {
  info "检查 npm ..."
  if ! command -v npm &>/dev/null; then
    warn "npm 未安装"
    return 1
  fi
  say "npm v$(npm -v) 已安装"
}

# --------------- install frontend deps ---------------
install_frontend_deps() {
  info "安装前端依赖 (npm install) ..."
  cd "$FRONTEND_DIR"
  npm install --silent
  say "前端依赖已安装 (node_modules)"
  cd "$PROJECT_ROOT"
}

# --------------- check all ---------------
check_prereqs() {
  local ok=true
  check_go || ok=false
  check_node || ok=false
  check_npm || ok=false
  if [ "$ok" = false ]; then
    err "请先安装缺失的依赖后再运行本脚本"
  fi
  say "所有前置依赖已就绪"
}

# --------------- main ---------------
main() {
  echo ""
  printf "${CYAN}╔══════════════════════════════════════╗${NC}\n"
  printf "${CYAN}║   oci-start 环境准备                 ║${NC}\n"
  printf "${CYAN}╚══════════════════════════════════════╝${NC}\n"
  echo ""

  check_prereqs
  echo ""

  info "创建数据目录 ..."
  mkdir -p "$PROJECT_ROOT/data/upload" "$PROJECT_ROOT/logs"
  say "data/ logs/ 已创建"
  echo ""

  install_frontend_deps
  echo ""

  printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
  printf "${GREEN}  环境准备完成 ✅${NC}\n"
  printf "${GREEN}  下一步: ./scripts/build.sh${NC}\n"
  printf "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

main "$@"
