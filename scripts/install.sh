#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-/usr/local}"
BINDIR="${BINDIR:-${PREFIX}/bin}"

install_bin() {
  local src="$1"
  local dst_name="$2"
  echo "[install] ${dst_name} -> ${BINDIR}"
  install -m 0755 "${src}" "${BINDIR}/${dst_name}"
}


install_bin "${ROOT_DIR}/components/droplet/bin/droplet" droplet
install_bin "${ROOT_DIR}/components/condenser/bin/condenser" condenser
install_bin "${ROOT_DIR}/components/condenser/bin/condenser-hook-agent" condenser-hook-agent
install_bin "${ROOT_DIR}/components/raind-cli/bin/raind" raind
